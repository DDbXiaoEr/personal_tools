/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule, TuiThemeCurrent } from "@opencode-ai/plugin/tui"
import type { AssistantMessage, Message } from "@opencode-ai/sdk/v2"
import type { RGBA } from "@opentui/core"
import { createEffect, createMemo, createSignal, For, on, onCleanup, Show } from "solid-js"
import { readFile } from "node:fs/promises"
import { homedir } from "node:os"
import { join } from "node:path"

type Totals = {
  input: number
  output: number
  reasoning: number
  cacheRead: number
  cacheWrite: number
  cost: number
}

const ZERO: Totals = {
  input: 0,
  output: 0,
  reasoning: 0,
  cacheRead: 0,
  cacheWrite: 0,
  cost: 0,
}

function todayStamp() {
  const d = new Date()
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, "0")
  const day = String(d.getDate()).padStart(2, "0")
  return `${y}-${m}-${day}`
}

function isToday(ms: number) {
  if (!ms) return false
  const d = new Date(ms)
  const n = new Date()
  return d.getFullYear() === n.getFullYear() && d.getMonth() === n.getMonth() && d.getDate() === n.getDate()
}

function add(a: Totals, b: Totals): Totals {
  return {
    input: a.input + b.input,
    output: a.output + b.output,
    reasoning: a.reasoning + b.reasoning,
    cacheRead: a.cacheRead + b.cacheRead,
    cacheWrite: a.cacheWrite + b.cacheWrite,
    cost: a.cost + b.cost,
  }
}

function maxTotal(a: Totals, b: Totals): Totals {
  return {
    input: Math.max(a.input, b.input),
    output: Math.max(a.output, b.output),
    reasoning: Math.max(a.reasoning, b.reasoning),
    cacheRead: Math.max(a.cacheRead, b.cacheRead),
    cacheWrite: Math.max(a.cacheWrite, b.cacheWrite),
    cost: Math.max(a.cost, b.cost),
  }
}

function fromMessage(m: AssistantMessage): Totals {
  const t = m.tokens
  if (!t) return ZERO
  return {
    input: t.input || 0,
    output: t.output || 0,
    reasoning: t.reasoning || 0,
    cacheRead: t.cache?.read || 0,
    cacheWrite: t.cache?.write || 0,
    cost: m.cost || 0,
  }
}

function totalsOf(values: Iterable<Totals>): Totals {
  let total = ZERO
  for (const v of values) total = add(total, v)
  return total
}

type SpeedStats = {
  live: number
  fastest: number
  slowest: number
  average: number
  samples: number
}

const NO_SPEED: SpeedStats = { live: 0, fastest: 0, slowest: 0, average: 0, samples: 0 }

function sessionSpeed(msgs: ReadonlyArray<Message>, now: number): SpeedStats {
  let fastest = 0
  let slowest = 0
  let tokens = 0
  let ms = 0
  let samples = 0
  let live = 0
  for (const m of msgs) {
    if (m.role !== "assistant") continue
    const a = m as AssistantMessage
    const created = a.time?.created || 0
    const completed = a.time?.completed || 0
    const count = (a.tokens?.output || 0) + (a.tokens?.reasoning || 0)
    if (!created || count <= 0) continue
    if (!completed) {
      const duration = now - created
      if (duration >= 400) live = count / (duration / 1000)
      continue
    }
    const duration = completed - created
    if (duration < 500) continue
    const speed = count / (duration / 1000)
    if (samples === 0 || speed > fastest) fastest = speed
    if (samples === 0 || speed < slowest) slowest = speed
    tokens += count
    ms += duration
    samples++
  }
  if (samples === 0 && live === 0) return NO_SPEED
  return {
    live,
    fastest,
    slowest,
    average: samples ? tokens / (ms / 1000) : 0,
    samples,
  }
}

type Balance = {
  amount: number
  currency: string
}

let balanceCache: (Balance & { at: number }) | null = null

function providerOf(messages: ReadonlyArray<Message>) {
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i]
    if (m.role === "assistant") return `${m.providerID}/${m.modelID}`
    if (m.role === "user" && m.model) return `${m.model.providerID}/${m.model.modelID}`
  }
  return ""
}

function isDeepseek(api: TuiPluginApi, session_id?: string) {
  let id = session_id ? providerOf(api.state.session.messages(session_id)) : ""
  if (!id) id = String(api.state.config.model || "")
  const provider = id.split("/")[0].toLowerCase()
  return provider === "deepseek" || provider.startsWith("deepseek-") || provider.startsWith("deepseek_")
}

async function deepseekKey() {
  const env = process.env.DEEPSEEK_API_KEY?.trim()
  if (env) return env
  try {
    const auth = JSON.parse(await readFile(join(homedir(), ".local/share/opencode/auth.json"), "utf8")) as Record<
      string,
      { key?: string }
    >
    for (const [id, entry] of Object.entries(auth || {})) {
      if (!id.toLowerCase().includes("deepseek")) continue
      const key = entry?.key?.trim()
      if (key) return key
    }
  } catch {
    // No auth file or invalid JSON: try the account store next.
  }
  try {
    const account = JSON.parse(
      await readFile(join(homedir(), ".local/share/opencode/account.json"), "utf8"),
    ) as {
      accounts?: Record<string, { serviceID?: string; credential?: { key?: string } }>
    }
    for (const entry of Object.values(account.accounts || {})) {
      if (!entry?.serviceID?.toLowerCase().includes("deepseek")) continue
      const key = entry.credential?.key?.trim()
      if (key) return key
    }
  } catch {
    // No account store: the balance simply stays hidden.
  }
  return null
}

async function fetchBalance(): Promise<Balance | null> {
  const key = await deepseekKey()
  if (!key) return null
  try {
    const res = await fetch("https://api.deepseek.com/user/balance", {
      headers: { Authorization: `Bearer ${key}`, Accept: "application/json" },
      signal: AbortSignal.timeout(15_000),
    })
    if (!res.ok) return null
    const data = (await res.json()) as {
      balance_infos?: Array<{ currency?: string; total_balance?: string }>
    }
    const info = data.balance_infos?.[0]
    const amount = Number(info?.total_balance)
    if (!info || !Number.isFinite(amount)) return null
    return { amount, currency: String(info.currency || "CNY") }
  } catch {
    return null
  }
}

function currencySymbol(currency: string) {
  if (currency === "CNY") return "¥"
  if (currency === "USD") return "$"
  return `${currency} `
}

function fullAmount(currency: string) {
  const configured = Number(process.env.DEEPSEEK_BALANCE_FULL)
  if (Number.isFinite(configured) && configured > 0) return configured
  return currency === "USD" ? 150 : 1000
}

function battery(ratio: number, width = 10) {
  const clamped = Math.min(1, Math.max(0, ratio))
  const filled = clamped <= 0 ? 0 : Math.max(1, Math.round(clamped * width))
  return `[${"█".repeat(filled)}${"░".repeat(width - filled)}]▌`
}

function BalanceLine(props: { api: TuiPluginApi; session_id?: string; version: () => number }) {
  const theme = () => props.api.theme.current
  const active = createMemo(() => {
    props.version()
    return isDeepseek(props.api, props.session_id)
  })
  const [info, setInfo] = createSignal<Balance | null>(balanceCache)
  const [loading, setLoading] = createSignal(!balanceCache)

  createEffect(
    on(active, (visible) => {
      if (!visible) return
      let disposed = false
      const load = async (force: boolean) => {
        const cached = balanceCache
        if (!force && cached && Date.now() - cached.at < 60_000) {
          setInfo(cached)
          setLoading(false)
          return
        }
        if (!cached) setLoading(true)
        const next = await fetchBalance()
        if (disposed) return
        if (next) {
          balanceCache = { ...next, at: Date.now() }
          setInfo(next)
        }
        setLoading(false)
      }
      void load(false)
      const timer = setInterval(() => void load(true), 60_000)
      onCleanup(() => {
        disposed = true
        clearInterval(timer)
      })
    }),
  )

  const ratio = () => {
    const value = info()
    if (!value) return 0
    return value.amount / fullAmount(value.currency)
  }
  const color = () => {
    const value = info()
    if (!value) return theme().textMuted
    const usd = value.currency === "USD"
    if (value.amount <= (usd ? 3 : 20)) return theme().error
    if (value.amount <= (usd ? 15 : 100)) return theme().warning
    return theme().success
  }
  const label = () => {
    const value = info()
    if (!value) return loading() ? "···" : "--"
    return `${currencySymbol(value.currency)}${value.amount.toFixed(2)}`
  }

  return (
    <Show when={active()}>
      <text>
        <span style={{ fg: theme().textMuted }}>BAL </span>
        <span style={{ fg: color() }}>{battery(ratio())}</span>
        <span style={{ fg: info() ? theme().text : theme().textMuted }}> {label()}</span>
      </text>
    </Show>
  )
}

type DbRow = Totals & { id: string }

let db: { rows: (sql: string, params?: string[]) => DbRow[] } | null = null
let dbFailed = false

function dbPath() {
  return join(homedir(), ".local/share/opencode/opencode.db")
}

async function getDb() {
  if (db || dbFailed) return db
  try {
    const { Database } = await import("bun:sqlite")
    const sqlite = new Database(dbPath(), { readonly: true })
    sqlite.run("PRAGMA query_only = ON")
    db = {
      rows(sql: string, params: string[] = []) {
        const list = sqlite.query(sql).all(...params) as Array<Record<string, unknown>>
        return list.map((r) => ({
          id: String(r.id),
          input: Number(r.input || 0),
          output: Number(r.output || 0),
          reasoning: Number(r.reasoning || 0),
          cacheRead: Number(r.cacheRead || 0),
          cacheWrite: Number(r.cacheWrite || 0),
          cost: Number(r.cost || 0),
        }))
      },
    }
  } catch {
    dbFailed = true
    db = null
  }
  return db
}

const SQL = `
SELECT
  id,
  ifnull(json_extract(data,'$.tokens.input'),0) as input,
  ifnull(json_extract(data,'$.tokens.output'),0) as output,
  ifnull(json_extract(data,'$.tokens.reasoning'),0) as reasoning,
  ifnull(json_extract(data,'$.tokens.cache.read'),0) as cacheRead,
  ifnull(json_extract(data,'$.tokens.cache.write'),0) as cacheWrite,
  ifnull(json_extract(data,'$.cost'),0) as cost
FROM message
WHERE json_extract(data,'$.role')='assistant'
  AND date(time_created/1000,'unixepoch','localtime')=?
`

type Store = {
  byId: Map<string, Totals>
  version: () => number
  peak: () => number
  trace: () => number[]
}

function fmt(n: number) {
  const v = Math.max(0, n)
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`
  return `${Math.round(v)}`
}

function fmtSpeed(n: number) {
  const v = Math.max(0, n)
  if (v >= 100) return `${Math.round(v)}`
  if (v >= 10) return v.toFixed(1)
  return v.toFixed(2)
}

function needleGlyph(angle: number) {
  const deg = (angle * 180) / Math.PI
  if (deg >= 157.5 || deg < 22.5) return "─"
  if (deg >= 112.5) return "╲"
  if (deg >= 67.5) return "│"
  return "╱"
}

type Cell = { ch: string; fg: RGBA }

function bandColor(t: number, theme: TuiThemeCurrent): RGBA {
  if (t >= 0.85) return theme.error
  if (t >= 0.62) return theme.warning
  return theme.success
}

function buildTach(speed: number, max: number, theme: TuiThemeCurrent): Cell[][] {
  const W = 21
  const H = 4
  const cx = 10
  const cy = 3
  const rx = 10
  const ry = 3
  const grid: Cell[][] = Array.from({ length: H }, () =>
    Array.from({ length: W }, () => ({ ch: " ", fg: theme.textMuted })),
  )

  const plot = (x: number, y: number, ch: string, fg: RGBA) => {
    if (x < 0 || x >= W || y < 0 || y >= H) return
    grid[y][x] = { ch, fg }
  }

  for (let a = 0; a <= 180; a += 4) {
    const rad = (a * Math.PI) / 180
    const x = Math.round(cx + rx * Math.cos(Math.PI - rad))
    const y = Math.round(cy - ry * Math.sin(rad))
    const t = a / 180
    const ch = a === 0 || a === 90 || a === 180 ? "•" : "·"
    plot(x, y, ch, t >= 0.85 ? theme.error : t >= 0.62 ? theme.warning : theme.secondary)
  }

  const t = Math.min(1, Math.max(0, speed / Math.max(max, 1)))
  const needleA = Math.PI * (1 - t)
  const color = bandColor(t, theme)
  const glyph = needleGlyph(needleA)
  for (let i = 1; i <= 8; i++) {
    const x = Math.round(cx + ((rx - 1) * i) / 8 * Math.cos(needleA))
    const y = Math.round(cy - (ry * i) / 8 * Math.sin(needleA))
    plot(x, y, i === 8 ? "◆" : glyph, color)
  }
  plot(cx, cy, "●", theme.primary)
  return grid
}

const SPARK = ["▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"]

function sparkline(values: number[], width: number) {
  if (!values.length) return "▁".repeat(width)
  const slice = values.slice(-width)
  const peak = Math.max(1, ...slice)
  const pad = Math.max(0, width - slice.length)
  return "▁".repeat(pad) + slice.map((v) => SPARK[Math.min(7, Math.floor((v / peak) * 7))]).join("")
}

function Gauge(props: { speed: number; max: number; live: boolean; theme: TuiThemeCurrent }) {
  const rows = createMemo(() => buildTach(props.speed, props.max, props.theme))
  return (
    <box>
      <For each={rows()}>
        {(row) => (
          <text>
            <For each={row}>{(cell) => <span style={{ fg: cell.fg }}>{cell.ch}</span>}</For>
          </text>
        )}
      </For>
    </box>
  )
}

function DailyLine(props: { api: TuiPluginApi; store: Store; session_id?: string }) {
  const theme = () => props.api.theme.current
  const data = createMemo(() => {
    props.store.version()
    return totalsOf(props.store.byId.values())
  })
  const speed = createMemo(() => {
    props.store.version()
    if (!props.session_id) return NO_SPEED
    return sessionSpeed(props.api.state.session.messages(props.session_id), Date.now())
  })
  const shown = () => speed().live || speed().average
  const live = () => speed().live > 0

  return (
    <text>
      <span style={{ fg: live() ? theme().success : theme().textMuted }}>{live() ? "● " : "○ "}</span>
      <span style={{ fg: bandColor(shown() / Math.max(props.store.peak(), 1), theme()) }}>
        {shown() ? fmtSpeed(shown()) : "-"}
      </span>
      <span style={{ fg: theme().textMuted }}> t/s  </span>
      <span style={{ fg: theme().primary }}>{fmt(data().input + data().output + data().reasoning)}</span>
    </text>
  )
}

function Cluster(props: { api: TuiPluginApi; store: Store; session_id?: string }) {
  const theme = () => props.api.theme.current
  const data = createMemo(() => {
    props.store.version()
    return totalsOf(props.store.byId.values())
  })
  const speed = createMemo(() => {
    props.store.version()
    if (!props.session_id) return NO_SPEED
    return sessionSpeed(props.api.state.session.messages(props.session_id), Date.now())
  })
  const shown = () => speed().live || speed().average
  const live = () => speed().live > 0
  const max = () => Math.max(80, props.store.peak(), 1)
  const trace = () => {
    props.store.version()
    return sparkline(props.store.trace(), 21)
  }

  return (
    <box>
      <BalanceLine api={props.api} session_id={props.session_id} version={props.store.version} />
      <text>
        <span style={{ fg: theme().primary }}>◆ TOTAL</span>
        <span style={{ fg: theme().textMuted }}> </span>
        <span style={{ fg: live() ? theme().success : theme().textMuted }}>{live() ? "● LIVE" : "○ LIVE"}</span>
      </text>
      <Gauge speed={shown()} max={max()} live={live()} theme={theme()} />
      <text>
        <span style={{ fg: theme().textMuted }}>0</span>
        <span style={{ fg: bandColor(shown() / max(), theme()) }}>
          {shown() ? `   ${fmtSpeed(shown())} t/s   ` : "     -- t/s     "}
        </span>
        <span style={{ fg: theme().textMuted }}>{Math.round(max())}</span>
      </text>
      <text>
        <span style={{ fg: live() ? theme().secondary : theme().textMuted }}>{trace()}</span>
      </text>
      <text>
        <span style={{ fg: theme().textMuted }}>ODO </span>
        <span style={{ fg: theme().primary }}>{fmt(data().input + data().output + data().reasoning)}</span>
        <span style={{ fg: theme().textMuted }}> tok</span>
      </text>
      <text>
        <span style={{ fg: theme().textMuted }}>IN </span>
        <span style={{ fg: theme().text }}>{fmt(data().input)}</span>
        <span style={{ fg: theme().textMuted }}>  OUT </span>
        <span style={{ fg: theme().text }}>{fmt(data().output)}</span>
      </text>
      <text>
        <span style={{ fg: theme().textMuted }}>CACHE </span>
        <span style={{ fg: theme().secondary }}>{fmt(data().cacheRead)}</span>
      </text>
    </box>
  )
}

const tui: TuiPlugin = async (api) => {
  const byId = new Map<string, Totals>()
  const [version, setVersion] = createSignal(0)
  const [peak, setPeak] = createSignal(80)
  const [trace, setTrace] = createSignal<number[]>([])
  const bump = () => setVersion((n) => n + 1)
  let day = todayStamp()

  const resync = async () => {
    const database = await getDb()
    if (!database) return
    try {
      for (const row of database.rows(SQL, [day])) {
        const existing = byId.get(row.id)
        byId.set(row.id, existing ? maxTotal(existing, row) : row)
      }
      bump()
    } catch {
    }
  }

  const rollover = () => {
    const next = todayStamp()
    if (next === day) return false
    day = next
    byId.clear()
    setTrace([])
    setPeak(80)
    void resync()
    return true
  }

  const store: Store = { byId, version, peak, trace }

  await resync()

  api.event.on("message.updated", (event) => {
    rollover()
    const info = event.properties.info
    if (info.role !== "assistant") return
    const a = info as AssistantMessage
    if (!isToday(a.time?.created || 0)) return
    byId.set(a.id, fromMessage(a))
    bump()
  })

  api.event.on("message.removed", (event) => {
    byId.delete(event.properties.messageID)
    bump()
  })

  let ticks = 0
  const timer = setInterval(() => {
    if (rollover()) return
    ticks++
    const route = api.route.current
    const sessionID = route.name === "session" ? String(route.params?.sessionID || "") : ""
    if (sessionID) {
      const stats = sessionSpeed(api.state.session.messages(sessionID), Date.now())
      const current = stats.live || stats.average
      if (current > peak()) setPeak(Math.ceil(current / 20) * 20)
      if (stats.fastest > peak()) setPeak(Math.ceil(stats.fastest / 20) * 20)
      setTrace((prev) => {
        const next = prev.concat(current)
        return next.length > 24 ? next.slice(-24) : next
      })
    }
    if (ticks % 150 === 0) void resync()
    bump()
  }, 200)
  api.lifecycle.onDispose(() => clearInterval(timer))

  api.slots.register({
    slots: {
      session_prompt_right(_ctx, props: { session_id: string }) {
        return <DailyLine api={api} store={store} session_id={props.session_id} />
      },
      home_prompt_right(_ctx) {
        return <DailyLine api={api} store={store} />
      },
      sidebar_footer(_ctx, props: { session_id: string }) {
        return <Cluster api={api} store={store} session_id={props.session_id} />
      },
    },
  })
}

export default { id: "daily-tokens", tui } satisfies TuiPluginModule

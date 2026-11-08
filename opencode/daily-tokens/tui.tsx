/** @jsxImportSource @opentui/solid */
import { Plugin } from "@opencode/plugin/tui"
import type { RGBA } from "@opentui/core"
import { createEffect, createMemo, createSignal, For, onCleanup, onMount } from "solid-js"
import { homedir } from "node:os"
import { join } from "node:path"

type Ctx = Plugin.Context
type Color = Ctx["theme"]["text"]["base"]
type Message = ReturnType<Ctx["data"]["session"]["message"]["list"]>[number]
type Assistant = Extract<Message, { type: "assistant" }>
type Session = ReturnType<Ctx["data"]["session"]["list"]>[number]

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

function fromSession(s: Session): Totals {
  const t = s.tokens
  if (!t) return ZERO
  return {
    input: t.input || 0,
    output: t.output || 0,
    reasoning: t.reasoning || 0,
    cacheRead: t.cache?.read || 0,
    cacheWrite: t.cache?.write || 0,
    cost: s.cost || 0,
  }
}

function startOfDay(date = new Date()) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
}

const DAY_MS = 24 * 60 * 60 * 1000

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

type Db = {
  totals(start: number, end: number): Totals
  close(): void
}

const SQL = `
SELECT
  sum(ifnull(json_extract(data,'$.tokens.input'),0)) AS input,
  sum(ifnull(json_extract(data,'$.tokens.output'),0)) AS output,
  sum(ifnull(json_extract(data,'$.tokens.reasoning'),0)) AS reasoning,
  sum(ifnull(json_extract(data,'$.tokens.cache.read'),0)) AS cacheRead,
  sum(ifnull(json_extract(data,'$.tokens.cache.write'),0)) AS cacheWrite,
  sum(ifnull(json_extract(data,'$.cost'),0)) AS cost
FROM session_message
WHERE type = 'assistant' AND time_created >= ? AND time_created < ?
`

async function openDb(): Promise<Db | null> {
  try {
    // @ts-ignore - Bun runtime builtin
    const { Database } = await import("bun:sqlite")
    const path = join(homedir(), ".local/share/opencode/opencode.db")
    const sqlite = new Database(path, { readonly: true })
    sqlite.run("PRAGMA query_only = ON")
    const query = sqlite.query(SQL)
    return {
      totals(start: number, end: number) {
        const row = (query.all(start, end) as Array<Record<string, unknown>>)[0] ?? {}
        return {
          input: Number(row.input || 0),
          output: Number(row.output || 0),
          reasoning: Number(row.reasoning || 0),
          cacheRead: Number(row.cacheRead || 0),
          cacheWrite: Number(row.cacheWrite || 0),
          cost: Number(row.cost || 0),
        }
      },
      close() {
        sqlite.close()
      },
    }
  } catch {
    return null
  }
}

type Speed = {
  live: number
  fastest: number
  average: number
  samples: number
}

const NO_SPEED: Speed = { live: 0, fastest: 0, average: 0, samples: 0 }

const SPARK = ["▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"]

function sparkline(values: number[], width: number, scale: number) {
  if (!values.length) return "▁".repeat(width)
  const slice = values.slice(-width)
  const peak = Math.max(1, scale, ...slice)
  const pad = Math.max(0, width - slice.length)
  return "▁".repeat(pad) + slice.map((v) => SPARK[Math.min(7, Math.floor((v / peak) * 7))]).join("")
}

function needleGlyph(angle: number) {
  const deg = (angle * 180) / Math.PI
  if (deg >= 157.5 || deg < 22.5) return "─"
  if (deg >= 112.5) return "╲"
  if (deg >= 67.5) return "│"
  return "╱"
}

type Cell = { ch: string; fg: Color }

function bandColor(t: number, colors: Colors): Color {
  if (t >= 0.85) return colors.error
  if (t >= 0.62) return colors.warning
  return colors.success
}

function buildTach(speed: number, max: number, colors: Colors): Cell[][] {
  const W = 21
  const H = 4
  const cx = 10
  const cy = 3
  const rx = 10
  const ry = 3
  const grid: Cell[][] = Array.from({ length: H }, () =>
    Array.from({ length: W }, () => ({ ch: " ", fg: colors.muted })),
  )

  const plot = (x: number, y: number, ch: string, fg: Color) => {
    if (x < 0 || x >= W || y < 0 || y >= H) return
    grid[y][x] = { ch, fg }
  }

  for (let a = 0; a <= 180; a += 4) {
    const rad = (a * Math.PI) / 180
    const x = Math.round(cx + rx * Math.cos(Math.PI - rad))
    const y = Math.round(cy - ry * Math.sin(rad))
    const t = a / 180
    const ch = a === 0 || a === 90 || a === 180 ? "•" : "·"
    plot(x, y, ch, t >= 0.85 ? colors.error : t >= 0.62 ? colors.warning : colors.secondary)
  }

  const t = Math.min(1, Math.max(0, speed / Math.max(max, 1)))
  const needleA = Math.PI * (1 - t)
  const color = bandColor(t, colors)
  const glyph = needleGlyph(needleA)
  for (let i = 1; i <= 8; i++) {
    const x = Math.round(cx + (((rx - 1) * i) / 8) * Math.cos(needleA))
    const y = Math.round(cy - ((ry * i) / 8) * Math.sin(needleA))
    plot(x, y, i === 8 ? "◆" : glyph, color)
  }
  plot(cx, cy, "●", colors.primary)
  return grid
}

type Colors = {
  text: Color
  muted: Color
  primary: Color
  secondary: Color
  success: Color
  warning: Color
  error: Color
}

function colorsOf(theme: Ctx["theme"]): Colors {
  return {
    text: theme.text.base,
    muted: theme.text.muted,
    primary: theme.text.action.primary.base,
    secondary: theme.text.action.secondary.base,
    success: theme.text.feedback.success.base,
    warning: theme.text.feedback.warning.base,
    error: theme.text.feedback.error.base,
  }
}

function contentChars(message: Assistant) {
  let chars = 0
  for (const item of message.content ?? []) {
    if (item.type === "text" || item.type === "reasoning") {
      chars += item.text?.length ?? 0
    } else if (item.type === "tool") {
      const input = item.state?.input
      if (input !== undefined) {
        try {
          chars += JSON.stringify(input).length
        } catch {
          // Ignore values that cannot be serialized.
        }
      }
    }
  }
  return chars
}

type LiveState = {
  samples: Array<{ at: number; n: number }>
  messageChars: Map<string, number>
  ratio: number
}

const WINDOW_MS = 1600
const INITIAL_RATIO = 0.3

function emptyLive(): LiveState {
  return { samples: [], messageChars: new Map(), ratio: INITIAL_RATIO }
}

const tui = Plugin.define({
  id: "daily-tokens",
  async setup(ctx) {
    const [version, setVersion] = createSignal(0)
    const [daily, setDaily] = createSignal<Totals>(ZERO)
    const [peak, setPeak] = createSignal(80)
    const [trace, setTrace] = createSignal<number[]>([])
    const [activeSession, setActiveSession] = createSignal("")
    const liveStates = new Map<string, LiveState>()
    const bump = () => setVersion((n) => n + 1)

    let dayStart = startOfDay()
    let resyncTimer: ReturnType<typeof setTimeout> | undefined
    let db: Db | null | undefined

    const database = async () => {
      if (db !== undefined) return db
      db = await openDb()
      return db
    }

    const liveState = (sessionID: string) => {
      let state = liveStates.get(sessionID)
      if (!state) {
        state = emptyLive()
        liveStates.set(sessionID, state)
      }
      return state
    }

    const recordChars = (sessionID: string, messageID: string, chars: number) => {
      if (!sessionID || chars <= 0) return
      const state = liveState(sessionID)
      const now = Date.now()
      state.samples.push({ at: now, n: chars })
      state.messageChars.set(messageID, (state.messageChars.get(messageID) ?? 0) + chars)
      const cutoff = now - WINDOW_MS * 2
      while (state.samples.length > 0 && state.samples[0].at < cutoff) state.samples.shift()
    }

    const liveSpeed = (sessionID: string, now: number) => {
      const state = liveStates.get(sessionID)
      if (!state) return 0
      const cutoff = now - WINDOW_MS
      while (state.samples.length > 0 && state.samples[0].at < cutoff) state.samples.shift()
      if (state.samples.length === 0) return 0
      let chars = 0
      for (const sample of state.samples) chars += sample.n
      const span = Math.max(0.5, Math.min(WINDOW_MS / 1000, (now - state.samples[0].at) / 1000))
      return (chars / span) * state.ratio
    }

    const speedOf = (sessionID: string, now: number): Speed => {
      if (!sessionID) return NO_SPEED
      const live = liveSpeed(sessionID, now)
      let tokens = 0
      let duration = 0
      let fastest = 0
      let samples = 0
      for (const message of ctx.data.session.message.list(sessionID) ?? []) {
        if (message.type !== "assistant") continue
        const created = message.time?.created || 0
        const completed = message.time?.completed || 0
        if (!created || !completed) continue
        const count = (message.tokens?.output || 0) + (message.tokens?.reasoning || 0)
        if (count <= 0) continue
        const elapsed = completed - created
        if (elapsed < 400) continue
        const speed = count / (elapsed / 1000)
        if (samples === 0 || speed > fastest) fastest = speed
        tokens += count
        duration += elapsed
        samples++
      }
      return {
        live,
        fastest,
        average: samples > 0 ? tokens / (duration / 1000) : 0,
        samples,
      }
    }

    const fallbackTotals = () => {
      const start = startOfDay()
      let total = ZERO
      for (const session of ctx.data.session.list()) {
        if ((session.time?.created || 0) < start) continue
        total = add(total, fromSession(session))
      }
      return total
    }

    const resync = async () => {
      const start = startOfDay()
      const end = start + DAY_MS
      const handle = await database()
      if (handle) {
        try {
          setDaily(handle.totals(start, end))
          return
        } catch {
          // A stale handle can be closed by a reload. Reopen on the next resync.
          try {
            handle.close()
          } catch {
            // Ignore a handle that is already closed.
          }
          db = undefined
        }
      }
      setDaily(fallbackTotals())
    }

    const scheduleResync = () => {
      if (resyncTimer) return
      resyncTimer = setTimeout(() => {
        resyncTimer = undefined
        void resync()
      }, 500)
    }

    const rollover = () => {
      const next = startOfDay()
      if (next === dayStart) return false
      dayStart = next
      setTrace([])
      setPeak(80)
      void resync()
      return true
    }

    await resync()

    const offs = [
      ctx.data.on("session.text.delta", (event) => {
        recordChars(event.data.sessionID, event.data.assistantMessageID, event.data.delta?.length ?? 0)
      }),
      ctx.data.on("session.reasoning.delta", (event) => {
        recordChars(event.data.sessionID, event.data.assistantMessageID, event.data.delta?.length ?? 0)
      }),
      ctx.data.on("session.tool.input.delta", (event) => {
        recordChars(event.data.sessionID, event.data.assistantMessageID, event.data.delta?.length ?? 0)
      }),
      ctx.data.on("session.step.ended", (event) => {
        const state = liveStates.get(event.data.sessionID)
        if (state) {
          const chars = state.messageChars.get(event.data.assistantMessageID) ?? 0
          const count = (event.data.tokens?.output || 0) + (event.data.tokens?.reasoning || 0)
          if (chars >= 40 && count > 0) {
            const observed = count / chars
            if (observed > 0.02 && observed < 1.5) state.ratio = state.ratio * 0.7 + observed * 0.3
          }
          state.messageChars.delete(event.data.assistantMessageID)
        }
        bump()
        scheduleResync()
      }),
      ctx.data.on("session.execution.succeeded", () => {
        bump()
        scheduleResync()
      }),
      ctx.data.on("session.execution.failed", () => {
        bump()
        scheduleResync()
      }),
    ]

    let ticks = 0
    const timer = setInterval(() => {
      if (rollover()) return
      ticks++
      const sessionID = activeSession()
      const now = Date.now()
      if (sessionID) {
        const stats = speedOf(sessionID, now)
        const current = stats.live || stats.average
        if (stats.live > peak()) setPeak(Math.ceil(stats.live / 20) * 20)
        if (stats.fastest > peak()) setPeak(Math.ceil(stats.fastest / 20) * 20)
        if (current > peak()) setPeak(Math.ceil(current / 20) * 20)
        setTrace((prev) => {
          const next = prev.concat(current)
          return next.length > 24 ? next.slice(-24) : next
        })
      }
      if (ticks % 150 === 0) void resync()
      bump()
    }, 200)

    const Gauge = (props: { speed: number; max: number; colors: Colors }) => {
      const rows = createMemo(() => buildTach(props.speed, props.max, props.colors))
      return (
        <box>
          <For each={rows()}>
            {(row) => (
              <text>
                <For each={row}>{(cell) => <span fg={cell.fg}>{cell.ch}</span>}</For>
              </text>
            )}
          </For>
        </box>
      )
    }

    const Cluster = (props: { sessionID: string }) => {
      let synced = ""
      createEffect(() => {
        const sessionID = props.sessionID
        setActiveSession(sessionID)
        if (sessionID && sessionID !== synced) {
          synced = sessionID
          void ctx.data.session.message.sync(sessionID).catch(() => undefined)
        }
      })
      onMount(() => {
        void ctx.data.session.message.sync(props.sessionID).catch(() => undefined)
      })
      onCleanup(() => {
        if (activeSession() === props.sessionID) setActiveSession("")
      })

      const colors = createMemo(() => {
        version()
        return colorsOf(ctx.theme)
      })
      const stats = createMemo(() => {
        version()
        return speedOf(props.sessionID, Date.now())
      })
      const totals = createMemo(() => {
        version()
        return daily()
      })
      const shown = () => stats().live || stats().average
      const live = () => stats().live > 0.5
      const max = () => Math.max(80, peak(), 1)
      const spark = createMemo(() => {
        version()
        return sparkline(trace(), 21, max())
      })

      return (
        <box>
          <text>
            <span fg={colors().primary}>◆ TOKEN</span>
            <span fg={colors().muted}>  </span>
            <span fg={live() ? colors().success : colors().muted}>{live() ? "● LIVE" : "○ LIVE"}</span>
          </text>
          <Gauge speed={shown()} max={max()} colors={colors()} />
          <text>
            <span fg={colors().muted}>0</span>
            <span fg={bandColor(shown() / max(), colors())}>
              {shown() ? `   ${fmtSpeed(shown())} t/s   ` : "     -- t/s     "}
            </span>
            <span fg={colors().muted}>{Math.round(max())}</span>
          </text>
          <text>
            <span fg={live() ? colors().secondary : colors().muted}>{spark()}</span>
          </text>
          <text>
            <span fg={colors().muted}>ODO </span>
            <span fg={colors().primary}>{fmt(totals().input + totals().output + totals().reasoning)}</span>
            <span fg={colors().muted}> tok</span>
          </text>
          <text>
            <span fg={colors().muted}>IN </span>
            <span fg={colors().text}>{fmt(totals().input)}</span>
            <span fg={colors().muted}>  OUT </span>
            <span fg={colors().text}>{fmt(totals().output)}</span>
          </text>
          <text>
            <span fg={colors().muted}>CACHE </span>
            <span fg={colors().secondary}>{fmt(totals().cacheRead)}</span>
          </text>
        </box>
      )
    }

    const slot = ctx.ui.slot({
      append: "sidebar.footer",
      render: ({ sessionID }) => <Cluster sessionID={sessionID} />,
    })

    return async () => {
      clearInterval(timer)
      if (resyncTimer) clearTimeout(resyncTimer)
      slot()
      for (const off of offs) off()
      try {
        db?.close()
      } catch {
        // Ignore a handle that is already closed.
      }
      db = undefined
    }
  },
})

export default tui

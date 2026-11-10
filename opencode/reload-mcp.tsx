/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginModule } from "@opencode-ai/plugin/tui"
import type { McpLocalConfig, McpRemoteConfig } from "@opencode-ai/sdk/v2"
import { execFile } from "node:child_process"
import { promisify } from "node:util"

const exec = promisify(execFile)

type McpEntry = McpLocalConfig | McpRemoteConfig

type ResolvedConfig = {
  mcp?: Record<string, McpEntry>
}

async function readResolvedConfig(directory: string): Promise<ResolvedConfig> {
  const candidates = [process.env.OPENCODE_BIN, process.execPath].filter((value): value is string => !!value)
  let lastError: unknown = new Error("Unable to locate the opencode executable")
  for (const executable of candidates) {
    try {
      const { stdout } = await exec(executable, ["--pure", "debug", "config"], {
        cwd: directory,
        maxBuffer: 16 * 1024 * 1024,
      })
      return JSON.parse(stdout) as ResolvedConfig
    } catch (error) {
      lastError = error
    }
  }
  throw lastError instanceof Error ? lastError : new Error(String(lastError))
}

async function reloadMcp(api: Parameters<TuiPlugin>[0]) {
  const directory = api.state.path.directory
  const toast = api.ui.toast
  toast({ variant: "info", message: "Reloading MCP servers...", duration: 2000 })

  let config: ResolvedConfig
  try {
    config = await readResolvedConfig(directory)
  } catch (error) {
    config = api.state.config as unknown as ResolvedConfig
    toast({
      variant: "warning",
      title: "Could not re-read opencode.json",
      message: `Using the config loaded at startup. ${error instanceof Error ? error.message : String(error)}`,
      duration: 5000,
    })
  }

  const servers = config.mcp ?? {}
  const before = await api.client.mcp.status({ directory })
  const names = new Set(Object.keys(before.data ?? {}))

  for (const [name, server] of Object.entries(servers)) {
    if (!server || typeof server !== "object") continue
    if (server.type !== "local" && server.type !== "remote") continue
    names.add(name)
    await api.client.mcp.add({ name, config: server, directory })
  }

  for (const name of names) {
    if (name in servers) continue
    await api.client.mcp.disconnect({ name, directory }).catch(() => undefined)
  }

  const after = await api.client.mcp.status({ directory })
  const statuses = Object.entries(after.data ?? {})
  const problems = statuses.filter(
    ([, status]) =>
      status.status === "failed" ||
      status.status === "needs_auth" ||
      status.status === "needs_client_registration",
  )

  if (problems.length > 0) {
    toast({
      variant: "warning",
      title: "MCP reloaded with errors",
      message: problems
        .map(([name, status]) => `${name}: ${"error" in status ? status.error : status.status}`)
        .join("\n"),
      duration: 8000,
    })
    return
  }

  const connected = statuses.filter(([, status]) => status.status === "connected").length
  toast({
    variant: "success",
    message: `Reloaded ${connected} connected MCP server(s)`,
    duration: 3000,
  })
}

const tui: TuiPlugin = async (api) => {
  api.command?.register(() => [
    {
      title: "Reload MCP servers",
      value: "mcp.reload",
      category: "MCP",
      description: "Re-read opencode.json and restart all MCP servers",
      slash: { name: "reload-mcp", aliases: ["reload_mcp"] },
      onSelect: () => reloadMcp(api),
    },
  ])
}

export default { id: "reload-mcp", tui } satisfies TuiPluginModule

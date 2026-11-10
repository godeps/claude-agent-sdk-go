# Embedding process supervision

`types.ClaudeAgentOptions` supports two host integration options:

- `WithCommandHook(func(*exec.Cmd) error)`: configure the prepared command immediately before it starts.
- `WithReplaceEnv(true)`: prevent inheritance of the parent environment.

For an isolated execution, use `WithEnv(allowedEnvironment).WithReplaceEnv(true)`. Existing `WithEnv` callers keep the original merge behavior unless replacement is explicitly enabled. SDK-managed variables and configured options are still applied. An empty replacement map does not inherit parent credentials.

The command hook sees the final environment, working directory and configured pipes. A host can configure a process group or a platform job object and retain the command for its supervisor. Return an error to reject the launch. Rejection cleans up pipes, temporary configuration and startup resources; a subsequent connection attempt does not mistake the rejected process for a live one.

The hook must not call `Start`, `Run` or `Wait`, or replace the SDK protocol pipes. The SDK owns process start and wait. Host process supervision must coordinate with the SDK lifecycle. Interrupt requests and disconnected HTTP clients do not by themselves confirm that a local process tree has stopped.

`CommandHook` is excluded from JSON serialization. Environment override debug logs identify variable names without logging their values.

The host command tests use actual OS subprocesses to verify the hook timing, explicit environment replacement, default merge compatibility and rejection cleanup. They do not make model-provider requests.

## End of stream

When the CLI closes stdout, the query response channel closes and pending control requests and callbacks are cancelled. A closed response channel does not create a ResultMessage. Hosts must treat a turn that ends without its native result as incomplete/unknown, rather than success or a user-confirmed cancellation. Message sends and closure are synchronized so a late tool callback cannot panic on a closed channel.

# Cloud Native Buildpack for Visual Studio Debugger

The Cloud Native Buildpack for Visual Studio Debugger provides the `vsdbg`
binary and sets it on the `$PATH` so that it may be invoked by subsequent
buildpacks or in the final running container.

## Integration

The CNB for Visual Studio Debugger provides `vsdbg` as a dependency. Downstream
buildpacks, like the [.NET Execute
CNB](https://github.com/paketo-buildpacks/dotnet-execute), can require the
`vsdbg` dependency by generating a [Build Plan
TOML](https://github.com/buildpacks/spec/blob/master/buildpack.md#build-plan-toml)
that resembles the following:


```toml
[[requires]]

  # The name of the Visual Studio Debugger dependency is "vsdbg". This value is considered
  # part of the public API for the buildpack and will not change without a plan
  # for deprecation.
  name = "vsdbg"

  # The buildpack supports some non-required metadata options.
  [requires.metadata]

    # Setting the launch flag to true will ensure that the Visual Studio
    # Debugger depdendency is available in the container at runtime. If you are
    # writing a buildpack that requires the presence of vsdbg at runtime, this
    # flag should be set to true.
    launch = true
```

The .NET Core language family buildpack supports the [inclusion of `vsdbg` in a
final image](https://paketo.io/docs/howto/dotnet-core/#enable-remote-debugging)
through the `BP_DEBUG_ENABLED` environment variable.

## Usage

To package this buildpack for consumption:

```
$ ./scripts/package.sh
```

This builds the buildpack's Go source using `GOOS=linux` by default. You can
supply another value as the first argument to `package.sh`.

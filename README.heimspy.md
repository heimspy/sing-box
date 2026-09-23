# Heimspy sing-box fork

This repository is a GitHub fork of [SagerNet/sing-box](https://github.com/SagerNet/sing-box).
The `heimspy` branch starts at upstream commit `0b8995879f29a9b98ee027bc17b75e101445b238`
and carries three focused commits: the Heimspy inspector integration, network startup
race fix, and SOCKS UDP race fix. Changes are ordinary Git commits, not embedded patches.

The inspector is in `service/heimspyinspector`; its README documents the controller
protocol. Keep upstream history and merge/rebase upstream updates deliberately.
The upstream remote is `https://github.com/SagerNet/sing-box.git`.

[heimspy/vscode](https://github.com/heimspy/vscode) pins this fork in
`sing-box.lock.json`, owns platform builds and license manifests, and bundles the
binary with [heimspy/agent](https://github.com/heimspy/agent) in VSIX releases.
The agent has an independent fork pin for its integration-test fixture; neither
repository depends on the retired core repository.

Run the maintained Go checks from this repository with Go 1.27.1:

```sh
bash scripts/test-heimspy.sh
```

The Heimspy inspector workflow runs race tests, vet and govulncheck on Linux,
macOS and Windows. No recursive submodule checkout is needed to build the CLI.
For reproducible distribution builds, follow the vscode repository README.
This fork retains sing-box's GPL-3.0 license and the inspector's original MIT notices.

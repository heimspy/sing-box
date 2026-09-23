# Heimspy sing-box fork

This repository is a GitHub fork of [SagerNet/sing-box](https://github.com/SagerNet/sing-box).
The `heimspy` branch starts at upstream commit `0b8995879f29a9b98ee027bc17b75e101445b238`
and carries three focused commits: the Heimspy inspector integration, network startup
race fix, and SOCKS UDP race fix. Changes are ordinary Git commits, not embedded patches.

The inspector is in `service/heimspyinspector`; its README documents the controller
protocol. Keep upstream history and merge/rebase upstream updates deliberately.
The upstream remote is `https://github.com/SagerNet/sing-box.git`.

[heimspy/core](https://github.com/heimspy/core) pins the tested commit and Go toolchain
and owns the platform build/test/release pipeline. [heimspy/agent](https://github.com/heimspy/agent)
controls this binary; [heimspy/vscode](https://github.com/heimspy/vscode) bundles it.

For a source checkout and reproducible build, follow the core repository README.
No recursive submodule checkout is needed to build the sing-box CLI/inspector.
This fork retains sing-box's GPL-3.0 license and the inspector's original MIT notices.

# Contributing to Onepanel CLI

## Building CLI Binaries
You can build CLI binaries as follows:

```
make all cli-version=0.8.0 \
    manifests-version=v0.8.0 \
    core-version=v0.8.0 \
    core-ui-version=v0.8.0
```

`cli-version` is the version of the CLI, following [SemVer](https://semver.org), example: `cli-version=0.8.0`

`manifests-version` is the release tag for [manifests](https://github.com/onepanelio/manifests)

`core-version` is the release tag for [core](https://github.com/onepanelio/core)

`core-ui-version` is the release tag for [core-ui](https://github.com/onepanelio/core-ui)
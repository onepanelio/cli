# Contributing to Onepanel CLI

## Building CLI Binaries
You can build CLI binaries as follows:

```
make all version=1.0.0 \
    manifests-version-tag=v1.0.0 \
    core-version-tag=v1.0.0 \
    core-ui-version-tag=v1.0.0
```

`cli-version` is the version of the CLI, following [SemVer](https://semver.org), example: `version=1.0.0`

`manifests-version-tag` is the release tag from [manifests](https://github.com/onepanelio/manifests/releases)

`core-version-tag` is the release tag from [core](https://github.com/onepanelio/core/releases)

`core-ui-version-tag` is the release tag from [core-ui](https://github.com/onepanelio/core-ui/releases)
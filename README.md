# cli

## Getting Started

First, run the `init` command. This will generate a sample configuration file.
At the moment, you must fill out the `manifestsRepo` key. 
This should be the directory where you have `onepanel/manifests`

You can then modify the generated `params.env` file with arguments you want.

## Config

The configuration file is stored in `.cli_config.yaml`.
If it does not exist, it is created the first time the `init` command is run.

There are two configuration options at the moment, both control where the manifests are loaded from.


### Github Manifest Loader (default)

Downloads the manifest from the Github repository. 

```
manifestSource:
  github:
    tag: latest  # Change this to use another tag
    overrideCache: false # This is optional. Only use this to always override your cache.
```

### Directory Manifest Loader

Copies the manifest from a local directory.

```
manifestSource:
  directory:
    folder: /path/to/manifests
    overrideCache: true # Use this to override the cache so you can make local changes and see them reflect here.
```

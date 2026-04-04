# Hooks for Provisioning Layers

The Azure Developer CLI supports running lifecycle hooks at the individual provisioning layer level when using [Layered Provisioning](../docs/feature-stages.md).

## Overview

In addition to project-level and service-level hooks, you can define `preprovision` and `postprovision` hooks directly on each entry in `infra.layers[]` inside `azure.yaml`. These layer hooks run in the context of the layer's own directory, making it easy to run layer-specific scripts without sharing infrastructure.

## Configuring Layer Hooks

Add a `hooks` block to any layer under `infra.layers` in your `azure.yaml`:

```yaml
infra:
  layers:
    - name: shared
      path: infra/shared
      hooks:
        preprovision:
          shell: sh
          run: ./scripts/prepare-shared.sh
        postprovision:
          shell: sh
          run: ./scripts/notify-shared-done.sh

    - name: app
      path: infra/app
      hooks:
        preprovision:
          shell: sh
          run: ./scripts/prepare-app.sh
```

### Supported hook events

| Hook name       | When it runs                           |
| --------------- | -------------------------------------- |
| `preprovision`  | Before the layer's resources are deployed |
| `postprovision` | After the layer's resources are deployed  |

### Hook paths

All paths specified in layer hooks are resolved **relative to the layer's `path`** directory (not the project root).

## Running Layer Hooks Manually

Use `azd hooks run` with the new `--layer` flag to run hooks for a specific provisioning layer:

```bash
# Run the 'preprovision' hook for the 'shared' layer only
azd hooks run preprovision --layer shared

# Run the 'postprovision' hook for the 'app' layer only
azd hooks run postprovision --layer app
```

> **Note**: `--layer` and `--service` are mutually exclusive. You cannot specify both in the same command.

## Multi-hook format

Each hook event accepts either a single hook object or an array of hook objects:

```yaml
infra:
  layers:
    - name: shared
      path: infra/shared
      hooks:
        preprovision:
          - shell: sh
            run: ./scripts/step1.sh
          - shell: sh
            run: ./scripts/step2.sh
```

## Related

- [Layered Provisioning](../docs/feature-stages.md)
- [Hooks (project and service level)](https://learn.microsoft.com/azure/developer/azure-developer-cli/azd-schema#hooks)

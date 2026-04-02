# Proxy Configuration

If behind a proxy server, `HTTP_PROXY` and `HTTPS_PROXY` and environment variables can be configured which will set the proxy that `azd` will use for all http/https requests.

The following examples illustrate using [Telerik Fiddler](https://www.telerik.com/fiddler) as a local proxy server.
After setting the below environment variables, you will start seeing requests within the fiddler trace output. 
An example `PROXY_ADDRESS` for fiddler would look like `127.0.0.1:8888`

Setting the environment variables to invalid values will result in various HTTP related error messages when running `azd` commands.

## Windows

```powershell
$env:HTTP_PROXY = <PROXY_ADDRESS>
$env:HTTPS_PROXY = <PROXY_ADDRESS>
```

## Linux / Mac OS

```bash
export HTTP_PROXY=<PROXY_ADDRESS>
export HTTPS_PROXY=<PROXY_ADDRESS>
```

## References

- [Go http package docs](https://pkg.go.dev/net/http)
- [StackOverflow](https://stackoverflow.com/questions/14661511/setting-up-proxy-for-http-client)

Per Go `net/http` package docs

> DefaultTransport is the default implementation of Transport and is used by DefaultClient. It establishes network connections as needed and caches them for reuse by subsequent calls. It uses HTTP proxies as directed by the environment variables HTTP_PROXY, HTTPS_PROXY and NO_PROXY (or the lowercase versions thereof).

## Docker Build Network

If your Docker builds require access to a network proxy or need host networking (for example, to resolve dependencies through a corporate proxy during `docker build`), you can configure the Docker network mode for a service in `azure.yaml`:

```yaml
services:
  web:
    docker:
      network: host
```

The `network` field maps to the `--network` flag of `docker build`. Common values are `host` (use the host's network stack) or `bridge` (default Docker bridge network). This is particularly useful when building containers in environments where internet access is only available through the host network.

# boilersuite

Boilersuite is a tool for checking license boilerplate in cert-manager projects. It was ported to Golang from a Python script originally written for Kubernetes. That Python script was also used [in cert-manager itself](https://github.com/cert-manager/cert-manager/blob/v1.11.0/hack/verify_boilerplate.py).

It uses boilerplate template files to specify how boilerplate should look in a variety of file formats including Makefiles, go files, Dockerfiles and bash scripts.

Since it's written in Go, it's easy to install in projects which already use Go (as is the case for cert-manager). Plus, it doesn't add a dependency on Python allowing for more compact CI environments which don't need to include Python.

## Boilerplate Templates

All templates are in `boilerplate-templates/` and can be changed as needed. The templates are embedded into the built Go binary to ensure portability.

Templates will be interpreted as:

- "suffix type" (e.g. `boilerplate.go.boilertmpl` will be used for `*.go` files)
- "prefix type" (e.g. `boilerplate.Dockerfile.boilertmpl` will be used for `Dockerfile` or `Dockerfile.*`)

All templates can be both types, e.g. `boilerplate.go.boilertmpl` will match `main.go` and `go.mod`. There are built-in
exceptions made for several files, including `go.mod`, `go.sum`, git directories and others.

## Validation Process

Assume in this example we're validating a go file, but the same applies to any supported file.

0. Load and parse all templates, replacing the `<<AUTHOR>>` marker with the configured author
1. Find the go template in the list of bundled templates
2. Check if the file has been generated or marked to be skipped. If so, skip.
3. Normalise the target file, removing shebang lines, Go build constraints and replacing dates with the `<<YEAR>>` marker.
4. Normalise spaces (e.g. Windows newlines, prefixed newlines) in the target file
5. Ensure the target file is at least as long as the template. If not, it can't possibly match and we error.
6. Ensure the target file starts with the template. If not, we error.

## Running

```console
boilersuite [--skip "paths to skip"] [--author "example"] [--verbose] <path-to-validate>
```

The `--author` parameter defaults to `cert-manager`.

The `--skip` parameter gives a list of space-separated paths which should not be validated.

The `--verbose` parameter prints output for every validated file or skipped directory.

The `<path-to-validate>` can either be a directory (which will be searched recursively) or a single file.

NB: The boilersuite repo (this repo!) includes a set of intentionally incorrect test fixtures under `fixture/`,
and so that directory needs to be skipped when validating this repo specifically. See `make validate-local-boilerplate`.

## Building

```console
make build
```

After this, boilersuite will be available at `_bin/boilersuite`.

## Testing

```console
make test-all
```

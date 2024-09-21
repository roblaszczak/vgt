<img align="right" width="300" src="docs/logo.svg">

# vgt - Visualise Go Test

`vgt` is a tool for visualising Go test results in a browser.

It's helpful with understanding parallelism of tests and identifying slow tests.
More information can be found in the [TODO] blog post.

![Screenshot 1](docs/img1.png)
![Screenshot 2](docs/img2.png)

## Installation

```
go install -u github.com/roblaszczak/vgt
```

You can also run without installing by running `go run github.com/roblaszczak/vgt@latest`.

## Usage

For visualising test results, run `go test` with the `-json` flag and pipe the output to `vgt`.

```
go test -json ./... | vgt
```

or with `go run`:

```
go test -json ./... | go run github.com/roblaszczak/vgt@latest
```

After tests were executed, a browser window will open with the visualisation.

If you want to preserve the output, you can pipe test logs to file and later pass it to `vgt`:

```
go test -json ./... > test.json
cat test.json | vgt
```


### Additional flags

```
Usage of vgt:
  -debug
    	enable debug mode
  -duration-cutoff string
    	threshold for test duration cutoff, under which tests are not shown in the chart (default "100µs")
  -keep-running
    	keep browser running after page was opened
  -pass-output
    	pass output received to stdout (default true)
  -print-html
    	print html to stdout instead of opening browser
```

## Development

If you have an idea for a feature or found a bug, feel free to open an issue or a pull request.

Before making a big change, it's a good idea to open an issue to discuss it first.

### Running tests

Tests are not really sophisticated, and are based on checking changes in golden files and checking in browser if
it looks good.

### Updating golden files

If you made a change and want to update golden files, you can run:

```
go test . -update-golden
```

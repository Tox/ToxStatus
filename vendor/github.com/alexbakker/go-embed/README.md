# go-embed

Tool for embedding binary data in Go software.

It recursively scans the given input directory and puts all of the files it
finds in a single .go file in base64 format. The embedded assets can be reached
by calling assets.Get(), which returns a map[string][]byte.

An example:

```go
assetMap := assets.GetAssets()
assetMap["css/bootstrap.min.css"]
```

Usage of the tool itself:

```
Usage of go-embed:
  -input string
    	input directory
  -output string
    	output file
  -pkg string
    	package name to use (default "assets")

```

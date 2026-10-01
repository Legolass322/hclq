# hclq

Command-line HCL processor. It reads HCL, applies a [jq](https://jqlang.org/) filter, and writes JSON or HCL.

```
hclq '.resource.aws_instance.web.ami' main.tf
```

## Install

```
go install github.com/Legolass322/hclq/cmd/hclq@latest
```

Or build from a checkout:

```
make build
```

## Usage

```
hclq [flags] [filter] [file...]
```

The first argument is the filter. Remaining arguments are files. With no files, hclq reads stdin. A pipe with no filter uses `.`.

```
hclq . config.hcl
hclq -r '.locals.name' config.hcl
hclq -c '.variable[].default' variables.tf
hclq -s '.[].name' a.hcl b.hcl
```

Flags:

```
-c           compact JSON, one value per line
-e           exit 1 if the last result is false or null
-f file      read the filter from a file
-H           print HCL expressions instead of JSON
-n           do not read input; the filter receives null
-r           print strings without JSON quotes
-s           read every input into one array
-version     print the version
```

Exit status: 0 success, 1 when `-e` rejects the last result, 2 usage, 3 the filter did not compile, 4 input did not parse, 5 the filter failed at runtime.

## Input shape

Attributes become object keys. A block is nested by its type and then by each label. One block at a given path is an object. Several blocks at the same path become an array.

```hcl
resource "aws_instance" "web" {
  ami   = "ami-123"
  count = 2
}

ingress {
  port = 80
}
ingress {
  port = 443
}
```

```json
{
  "ingress": [
    {"port": 80},
    {"port": 443}
  ],
  "resource": {
    "aws_instance": {
      "web": {
        "ami": "ami-123",
        "count": 2
      }
    }
  }
}
```

Literals are evaluated. `1 + 2` becomes `3`. An expression that needs variables or functions is kept as its source text, so `var.region` stays `"var.region"`.

JSON input is accepted as-is when the document starts with `{` or `[`.

`-H` prints the result as HCL expressions. Nested objects stay objects (`tags = { Name = "web" }`). That is the data view of the file, not a Terraform-formatted round trip.

## License

MIT. See [LICENSE](LICENSE).

Filters run through [gojq](https://github.com/itchyny/gojq).

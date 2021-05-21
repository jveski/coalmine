# Coalmine

Get features into production safely using canaries.


## Usage

See ./example for a complete example.

```go
const regionKey coalmine.Key = "region"

var myFeature = coalmine.NewFeature("myFeature",
	coalmine.WithExactMatch(regionKey, "westus"))

ctx := coalmine.WithValue(context.Background(), regionKey, "westus")
if coalmine.Enabled(ctx) {
	// enabled!
}
```

# spotify-endsong-artwork

## JSON to NDJSON

```console
$ cat sorted_streams.json | jq -c '.[]' > sorted_streams_ndjson.json
```

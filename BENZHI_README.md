# Tidegate

潮汐船闸水位联锁

## Build

```bash
export GOTOOLCHAIN=local
go build ./...
```

## Test

```bash
export GOTOOLCHAIN=local
go test ./... -count=1
```

## Docker (benzhi)

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh tidegate linux/amd64
./build_benzhi_docker.sh tidegate linux/arm64
```

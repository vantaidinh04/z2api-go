# Z2api Go

Proxy API for Z.ai compatible with OpenAI and Anthropic, written in Go.

## Installation

### Using Go

```bash
git clone https://github.com/Tyler-Dinh/z2api-go.git
cd z2api-go
go mod download
go run main.go
```

### Using Docker

```bash
git clone https://github.com/Tyler-Dinh/z2api-go.git
cd z2api-go
docker-compose up -d
```

## Configuration

Copy the `.env.example` file to `.env` and edit:

```bash
cp .env.example .env
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TOKEN` | Z.ai token (leave empty for anonymous mode, set for authenticated mode) | - |
| `PORT` | Server port | `8080` |
| `DEBUG` | Enable debug mode | `false` |
| `DEBUG_MSG` | Enable debug messages | `false` |
| `THINK_TAGS_MODE` | Thinking tags processing mode (`reasoning`, `think`, `strip`, `details`) | `reasoning` |
| `MODEL` | Default model | `glm-4.6` |

## License

MIT License
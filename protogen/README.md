# protogen

Генератор контроллеров из `.proto` файлов. Парсит proto-определения сервисов и создаёт Go-заглушки контроллеров с fx-модулями.

## Установка

```bash
go install github.com/vovanwin/platform/protogen/cmd/protogen@latest
```

## Параметры CLI

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `-api` | `./api` | Путь к директории с `.proto` файлами |
| `-output` | `./internal/controller` | Путь для генерации контроллеров |
| `-server-pkg` | `github.com/vovanwin/platform/server` | Import path пакета server |

## Использование

### Структура proto файлов

```
api/
  users/
    users.proto
  orders/
    orders.proto
```

Каждый `.proto` файл должен содержать `option go_package` и определение `service`:

```protobuf
syntax = "proto3";

option go_package = "github.com/myproject/pkg/users;users";

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
}
```

### Запуск генерации

```bash
# С дефолтными параметрами
protogen

# С кастомными путями
protogen -api ./proto -output ./internal/handlers
```

### Результат

Для `UserService` из примера выше генерируются файлы:

```
internal/controller/
  user/
    controller.go    — структура UserGRPCServer с зависимостями
    module.go        — fx-модуль для DI-регистрации
    get_user.go      — заглушка метода GetUser
    create_user.go   — заглушка метода CreateUser
```

Генератор **не перезаписывает** существующие файлы — создаёт только отсутствующие.

### Пример сгенерированного контроллера

```go
// controller.go
type UserGRPCServer struct {
    users.UnimplementedUserServiceServer
}

// get_user.go
func (s *UserGRPCServer) GetUser(ctx context.Context, req *users.GetUserRequest) (*users.GetUserResponse, error) {
    // TODO: implement
    return nil, status.Errorf(codes.Unimplemented, "not implemented")
}
```

### Пример сгенерированного модуля

```go
// module.go
func NewModule() fx.Option {
    return fx.Module("user",
        fx.Provide(NewUserGRPCServer),
        fx.Invoke(func(s *grpc.Server, srv *UserGRPCServer) {
            users.RegisterUserServiceServer(s, srv)
        }),
    )
}
```

## Как работает

1. Ищет все `*.proto` файлы в поддиректориях `-api`
2. Парсит `go_package`, `service` и `rpc` определения через regex
3. Генерирует структуру контроллера, fx-модуль и заглушки методов
4. Создаёт файлы только если они отсутствуют (`writeIfMissing`)

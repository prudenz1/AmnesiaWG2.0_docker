# AmneziaWG 2.0 Docker Image

Docker-образ AmneziaWG 2.0, собираемый из официальных исходников:

- `https://github.com/amnezia-vpn/amneziawg-go`
- `https://github.com/amnezia-vpn/amneziawg-tools`

## Что внутри

- `amneziawg-go` (userspace backend)
- `awg`, `awg-quick` (из `amneziawg-tools`)
- `entrypoint.sh` для bootstrap `wg0.conf`
- встроенный REST API (`awg-api`) на `:8080` с Bearer-токеном

## Быстрый старт

1. Создайте `.env` на основе `.env.example` и заполните:
   - `SERVER_URL` (ваш IP/домен)
   - `API_TOKEN` (длинный случайный секрет)
   - при необходимости `SERVER_PORT`
2. Запустите:

```bash
docker compose up -d --build
```

3. Проверьте:

```bash
docker ps
docker logs amneziawg
```

Web UI будет доступен через `nginx` на порту `80`.

## REST API

Все запросы требуют заголовок:

```text
Authorization: Bearer <API_TOKEN>
```

### Статус

- `GET /api/status`

### Пиры

- `GET /api/peers`
- `POST /api/peers`
- `DELETE /api/peers/{publicKey}`
- `GET /api/peers/{publicKey}/config`
- `GET /api/peers/{publicKey}/qr`

### Reload

- `POST /api/reload`

## Примеры

Создать пира:

```bash
curl -sS -X POST "http://127.0.0.1/api/peers" \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"phone","persistentKeepalive":25}'
```

Список пиров:

```bash
curl -sS "http://127.0.0.1/api/peers" \
  -H "Authorization: Bearer your-secret-token"
```

Получить конфиг:

```bash
curl -sS "http://127.0.0.1/api/peers/<PUBLIC_KEY>/config" \
  -H "Authorization: Bearer your-secret-token"
```

Получить QR (PNG):

```bash
curl -sS "http://127.0.0.1/api/peers/<PUBLIC_KEY>/qr" \
  -H "Authorization: Bearer your-secret-token" \
  --output peer.png
```

## Параметры AWG 2.0

Поддерживаются переменные:

- `AWG_JC`, `AWG_JMIN`, `AWG_JMAX`
- `AWG_S1`, `AWG_S2`, `AWG_S3`, `AWG_S4`
- `AWG_H1`, `AWG_H2`, `AWG_H3`, `AWG_H4`
- `AWG_I1`, `AWG_I2`, `AWG_I3`, `AWG_I4`, `AWG_I5`

Они используются при первичной генерации `/etc/amnezia/amneziawg/wg0.conf`.

## Безопасность и git

- Внешне открыты только `80/tcp` (`nginx`) и `51840/udp` (VPN).
- `amneziawg` API порт `8080` и `webui` порт `3000` доступны только внутри docker-сети.
- `nginx` добавляет безопасные заголовки и базовый rate limit.
- В `.gitignore` исключены чувствительные файлы (`config/wg0.conf`, содержимое `state/`, `.env`).
- Перед первым коммитом убедитесь, что в репозитории нет реальных ключей и токенов.

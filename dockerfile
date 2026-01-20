# Build Stage (Derleme Aşaması)
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Bağımlılıkları indir
COPY go.mod go.sum ./
RUN go mod download

# Kaynak kodları kopyala
COPY . .

# Binary dosyayı derle (Adı: go-smith)
RUN go build -o go-smith .

# Runtime Stage (Çalışma Aşaması - Hafif Sürüm)
FROM alpine:latest

WORKDIR /root/

# Build aşamasından binary'yi al
COPY --from=builder /app/go-smith .

# Config dosyasını ve klasörünü kopyala (ÖNEMLİ!)
COPY --from=builder /app/config ./config

# Portu aç
EXPOSE 8080

# Uygulamayı başlat
CMD ["./go-smith"]
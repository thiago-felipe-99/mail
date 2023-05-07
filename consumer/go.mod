module github.com/thiago-felipe-99/mail/consumer

go 1.20

require (
	github.com/allegro/bigcache/v3 v3.1.0
	github.com/go-playground/validator/v10 v10.12.0
	github.com/knadh/koanf/parsers/dotenv v0.1.0
	github.com/knadh/koanf/providers/env v0.1.0
	github.com/knadh/koanf/providers/file v0.1.0
	github.com/knadh/koanf/providers/structs v0.1.0
	github.com/knadh/koanf/v2 v2.0.0
	github.com/microcosm-cc/bluemonday v1.0.23
	github.com/minio/minio-go/v7 v7.0.50
	github.com/prometheus/client_golang v1.14.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/thiago-felipe-99/mail/rabbit v0.0.0-00010101000000-000000000000
	github.com/wneessen/go-mail v0.3.9
)

require (
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.16.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/leodido/go-urn v1.2.2 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/rabbitmq/amqp091-go v1.8.0 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)

replace github.com/thiago-felipe-99/mail/rabbit => ../rabbit

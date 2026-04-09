.PHONY: dev build run docker-build docker-up docker-down test

# 本地开发
dev:
	cd internal/gateway && go run .

# 编译
build:
	cd internal/gateway && go build -o gateway .

# Docker 本地运行
docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# 健康检查
health:
	curl -s http://localhost:8080/health | jq .

# 测试流式调用（需要设置 BRIDGE_KEY 环境变量）
test-stream:
	curl -N http://localhost:8080/v1/chat/completions \
	  -H "Authorization: Bearer $$BRIDGE_KEY" \
	  -H "Content-Type: application/json" \
	  -d '{"model":"smart-fast","messages":[{"role":"user","content":"hello"}],"stream":true}'

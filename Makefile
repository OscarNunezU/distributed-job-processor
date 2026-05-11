REGISTRY_HOST=localhost:5050       # usado desde el host para push
REGISTRY_CLUSTER=job-processor-registry:5000  # usado desde los pods para pull

# ── Protobuf ──────────────────────────────────────────────────────────────────
proto:
	mkdir -p query-service/gen/job/v1
	protoc \
		--proto_path=proto \
		--go_out=query-service/gen/job/v1 \
		--go_opt=paths=source_relative \
		--go-grpc_out=query-service/gen/job/v1 \
		--go-grpc_opt=paths=source_relative \
		proto/job.proto

# ── Kubernetes cluster ────────────────────────────────────────────────────────
cluster-create:
	k3d cluster create job-processor \
	  --agents 2 \
	  --port "3000:80@loadbalancer" \
	  --port "50051:50051@loadbalancer" \
	  --registry-create job-processor-registry:5050

cluster-delete:
	k3d cluster delete job-processor

# ── Images ────────────────────────────────────────────────────────────────────
images-build:
	docker build -t $(REGISTRY_HOST)/api:latest ./api
	docker build -t $(REGISTRY_HOST)/worker:latest ./worker
	docker build -t $(REGISTRY_HOST)/query-service:latest ./query-service

images-push:
	docker push $(REGISTRY_HOST)/api:latest
	docker push $(REGISTRY_HOST)/worker:latest
	docker push $(REGISTRY_HOST)/query-service:latest

# ── Deploy ────────────────────────────────────────────────────────────────────
deploy:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/secrets/
	kubectl apply -f k8s/postgres/
	kubectl apply -f k8s/rabbitmq/
	kubectl rollout status statefulset/postgres -n job-processor --timeout=60s
	kubectl rollout status statefulset/rabbitmq -n job-processor --timeout=60s
	kubectl apply -f k8s/api/
	kubectl apply -f k8s/worker/
	kubectl apply -f k8s/query-service/

undeploy:
	kubectl delete -f k8s/ --recursive --ignore-not-found

# ── Full local setup ──────────────────────────────────────────────────────────
up: images-build images-push deploy

status:
	kubectl get pods,svc,ingress,hpa -n job-processor

# Port-forward for local access (standard practice for K8s local dev)
port-forward:
	kubectl port-forward svc/api 3000:80 -n job-processor &
	kubectl port-forward svc/query-service 50051:50051 -n job-processor &
	@echo "API  → http://localhost:3000"
	@echo "gRPC → localhost:50051"

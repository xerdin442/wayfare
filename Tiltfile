# Load the restart_process extension
load('ext://restart_process', 'docker_build_with_restart')

### K8s Config ###

local('kubectl create secret generic app-secrets --from-env-file=.env --dry-run=client -o yaml > ./infra/development/k8s/secrets.yaml')

k8s_yaml('./infra/development/k8s/secrets.yaml')
k8s_yaml('./infra/development/k8s/redis.yaml')
k8s_yaml('./infra/development/k8s/mongo.yaml')

### End of K8s Config ###
### API Gateway ###

docker_build_with_restart(
  'wayfare/api-gateway',
  '.',
  entrypoint=['/app/build/api-gateway'],
  dockerfile='./infra/development/docker/api-gateway.Dockerfile',
  ignore=['./infra', './tools'],
  live_update=[
    sync('./services/api-gateway', '/app/services/api-gateway'),
    sync('./shared', '/app/shared'),
    run(
      'go build -o /app/build/api-gateway ./services/api-gateway/cmd/.',
      trigger=['./services/api-gateway', './shared']
    )
  ],
)

k8s_yaml('./infra/development/k8s/api-gateway-deployment.yaml')
k8s_resource(
  'api-gateway',
  port_forwards=8080,
  resource_deps=['redis'],
  labels="services",
)

### End of API Gateway ###
### Trip Service ###

docker_build_with_restart(
  'wayfare/trip-service',
  '.',
  entrypoint=['/app/build/trip'],
  dockerfile='./infra/development/docker/trip-service.Dockerfile',
  ignore=['./infra', './tools'],
  live_update=[
    sync('./services/trip', '/app/services/trip'),
    sync('./shared', '/app/shared'),
    run(
      'go build -o /app/build/trip ./services/trip/cmd/main.go',
      trigger=['./services/trip', './shared']
    )
  ],
)

k8s_yaml('./infra/development/k8s/trip-service-deployment.yaml')
k8s_resource(
  'trip-service',
  resource_deps=['mongodb'],
  labels="services",
)

### End of Trip Service ###
# Load the restart_process extension
load('ext://restart_process', 'docker_build_with_restart')

### K8s Config ###

local('kubectl create secret generic app-secrets --from-env-file=.env --dry-run=client -o yaml > ./infra/development/k8s/secrets.yaml')

k8s_yaml('./infra/development/k8s/secrets.yaml')

k8s_yaml('./infra/development/k8s/redis.yaml')
k8s_resource('redis', labels="infra")

k8s_yaml('./infra/development/k8s/mongo.yaml')
k8s_resource('mongodb', port_forwards=['27017:27017'], labels="infra")

k8s_yaml('./infra/development/k8s/rabbitmq.yaml')
k8s_resource('rabbitmq', port_forwards=['15672:15672'], labels="infra")

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
  resource_deps=['redis', 'rabbitmq'],
  labels="services",
)

### End of API Gateway ###
### Trip Service ###

docker_build_with_restart(
  'wayfare/trip-service',
  '.',
  entrypoint=['/app/build/trip-service'],
  dockerfile='./infra/development/docker/trip-service.Dockerfile',
  ignore=['./infra', './tools'],
  live_update=[
    sync('./services/trip', '/app/services/trip'),
    sync('./shared', '/app/shared'),
    run(
      'go build -o /app/build/trip-service ./services/trip/cmd/main.go',
      trigger=['./services/trip', './shared']
    )
  ],
)

k8s_yaml('./infra/development/k8s/trip-service-deployment.yaml')
k8s_resource(
  'trip-service',
  resource_deps=['mongodb', 'rabbitmq'],
  labels="services",
)

### End of Trip Service ###
### Driver Service ###

docker_build_with_restart(
  'wayfare/driver-service',
  '.',
  entrypoint=['/app/build/driver-service'],
  dockerfile='./infra/development/docker/driver-service.Dockerfile',
  ignore=['./infra', './tools'],
  live_update=[
    sync('./services/driver', '/app/services/driver'),
    sync('./shared', '/app/shared'),
    run(
      'go build -o /app/build/driver-service ./services/driver/cmd/main.go',
      trigger=['./services/driver', './shared']
    )
  ],
)

k8s_yaml('./infra/development/k8s/driver-service-deployment.yaml')
k8s_resource(
  'driver-service',
  resource_deps=['redis', 'mongodb', 'rabbitmq'],
  labels="services",
)

### End of Driver Service ###
### Rider Service ###

docker_build_with_restart(
  'wayfare/rider-service',
  '.',
  entrypoint=['/app/build/rider-service'],
  dockerfile='./infra/development/docker/rider-service.Dockerfile',
  ignore=['./infra', './tools'],
  live_update=[
    sync('./services/rider', '/app/services/rider'),
    sync('./shared', '/app/shared'),
    run(
      'go build -o /app/build/rider-service ./services/rider/cmd/main.go',
      trigger=['./services/rider', './shared']
    )
  ],
)

k8s_yaml('./infra/development/k8s/rider-service-deployment.yaml')
k8s_resource(
  'rider-service',
  resource_deps=['mongodb', 'rabbitmq'],
  labels="services",
)

### End of Rider Service ###
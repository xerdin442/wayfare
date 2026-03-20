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

# Uncomment once we have a trip service

#trip_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/trip-service ./services/trip-service/cmd/main.go'
#if os.name == 'nt':
#  trip_compile_cmd = './infra/development/docker/trip-build.bat'

# local_resource(
#   'trip-service-compile',
#   trip_compile_cmd,
#   deps=['./services/trip-service', './shared'], labels="compiles")

# docker_build_with_restart(
#   'wayfare/trip-service',
#   '.',
#   entrypoint=['/app/build/trip-service'],
#   dockerfile='./infra/development/docker/trip-service.Dockerfile',
#   only=[
#     './build/trip-service',
#     './shared',
#   ],
#   live_update=[
#     sync('./build', '/app/build'),
#     sync('./shared', '/app/shared'),
#   ],
# )

# k8s_yaml('./infra/development/k8s/trip-service-deployment.yaml')
# k8s_resource('trip-service', resource_deps=['trip-service-compile'], labels="services")

### End of Trip Service ###
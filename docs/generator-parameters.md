# Input File

HydraGen uses a standard taxonomy with hierarchical structure, which is the input to the Generator module. The input to this module must be given in JSON format following the taxonomy described here.

## Overall Application Taxonomy

At least one cluster and endpoint is required. Other sections are optional and will be set to their default values if omitted.

#### Required attributes

* **name**: The name of the service object in Kubernetes.
* **protocol**. Determines if endpoints should respond to HTTP or gRPC requests.
* **clusters**: An array of clusters that the service will be deployed on.
* **endpoints**: An array of HTTP/gRPC endpoints that the service exposes.

#### Optional attributes

* **logging**: Enables logging using Elasticsearch. See [logging.md](logging.md) for more information.
* **development**: Builds the application emulator from a local source image (`hydragen-base`) instead of the latest release image.
* **base_image**: Specifies the base Docker image for the application emulator. For example, to use Ubuntu 20.04, set this to `ubuntu:20.04`. The default is `busybox` which provides a minimal shell and set of utilities.
* **resources**: Resource allocation requests and limits.
* **processes**: The maximum number of processes the service is allowed to use (`GOMAXPROCS`). If this is set to 0, the Go runtime will choose the number of processes to use. Default: 0
* **readiness_probe**: The initial delay before readiness probe is initiated. Default: 1 second

#### Format

```json
{
  "settings": {
    "logging": <boolean>,
    "development": <boolean>,
    "base_image": "<string>"
  },
  "services": [
    {
      "name": "<string>",
      "protocol": "<string:http|grpc>",
      "clusters": [...],
      "resources": {...},
      "processes": <integer>,
      "readiness_probe": <integer:seconds>,
      "endpoints": [...]
    }
  ],
  ...
}
```

## Describing Workload Placement and Scaling

The user can define the placement of each microservice on a specific node, cluster, or namespace. It is also possible to specify the number of microservice replicas once deployed.

#### Required attributes

* **cluster**: The cluster that the service will be deployed on.

#### Optional attributes

* **node**: Constrain the service to run on a specific node (for example, "cluster1-worker1"). Default: Empty string
* **namespace**: The namespace that the service will be created in. Default: "default"
* **replicas**: The number of replicas to create. Default: 1
* **annotations**: An array of arbitrary metadata to attach to the service. Default: Empty array

#### Format

```json
"clusters": [
    {
      "cluster": "<string>",
      "node": "<string>",
      "namespace": "<string>",
      "replicas": <integer>,
      "annotations": [...]
    },
    ...
]
```

## Describing Resource Allocation

HydraGen also supports the configuration of the requested resources to be allocated to a microservice instance and the maximum resource usage in terms of both CPU and memory.

#### Optional attributes

* **limits/cpu**: The maximum amount of CPU time that the service can use. Default: 1000m
* **limits/memory**: The maximum amount of memory that the service can use. Default: 1024M
* **requests/cpu**: The desired amount of CPU time for the service. Default: 500m
* **requests/memory**: The desired amount of memory for the service. Default: 256M

```json
"resources": {
  "limits": {
    "cpu": "<string:cores>",
    "memory": "<string:bytes>",
  },
  "requests": {
    "cpu": "<string:cores>",
    "memory": "<string:bytes>"
  }
}
```

## Describing Topological Architecture

For each microservice, HydraGen supports a set of configuration parameters that define the topological architecture of an application by describing the dependencies between services. To define the microservice fan-in, different parameters can be used which specify the set of endpoints a component serves. For each endpoint, the user can specify parameters such as a relative fan-out based on a set of calls to subsequent microservice endpoints as well as the execution mode across these calls. These options enable the user to generate complex multi-tier application architectures with different fan-in and/or fan-out characteristics.

#### Required attributes

* **name**: The request path of the endpoint. Can only contain lowercase alphanumeric characters, '.' or '-'.

#### Optional attributes

* **execution_mode**: Determines if the server responding at this endpoint should handle requests sequentially or in parallel, on multiple threads. Default: "sequential"
* **cpu_complexity**: CPU stress parameters.
* **network_complexity**: Network stress parameters.
* **resilience_patterns**: Resilience patterns parameters.

#### Format

```json
"endpoints": [
  {
    "name": "<string>",
    "execution_mode": "<string:sequential|parallel>",
    "resilience_patterns": {...},
    "cpu_complexity": {...},
    "network_complexity": {...}
  },
  ...
]
```

# Describing Resilience Patterns

HydraGen supports the injection of resilience patterns into the generated client code. Resilience mechanisms can be configured at two different levels:

- **Endpoint level:** patterns that affect all outgoing calls performed by an endpoint.
- **Called service level:** patterns applied only to a specific downstream dependency.

Currently, HydraGen supports:

- Circuit Breaker
- Exponential Backoff
- Timeout
- Fallback

> **Note:** Resilience patterns are mutually exclusive — only one pattern can be active per service call at a time. The only exception is Fallback, which can be combined with any of the other patterns.

## Endpoint-Level Resilience

Endpoint-level resilience patterns are configured under the endpoint definition.

### Circuit Breaker

The circuit breaker implementation uses three states: _CLOSED_, _OPEN_, and _HALF_OPEN_.

If a request exceeds the configured timeout, the circuit transitions to the _OPEN_ state and subsequent requests are rejected. After the configured retry interval, the circuit enters the _HALF_OPEN_ state and allows a test request to determine whether communication with the downstream service can be restored.

The circuit breaker is configured at endpoint level, but it must be explicitly enabled for each downstream service call using the `active_circuit_breaker` attribute.

#### Required attributes

* **timeout**: Determines the timeout in seconds to consider the response as failed.
* **retry_timer**: Determines the number of seconds that the circuit breaker must wait to retry a request to the destination service connection.

#### Format

```json
"resilience_patterns": {
  "circuit_breaker": {
    "timeout": <integer>,
    "retry_timer": <integer>
  }
}
```

#### Example

```json
"endpoints": [
  {
    "name": "api",
    "resilience_patterns": {
      "circuit_breaker": {
        "timeout": 5,
        "retry_timer": 10
      }
    }
  }
]
```

## Called Service Resilience

HydraGen also supports resilience mechanisms that are configured independently for each downstream service call. These patterns are defined inside a `called_service` entry.

#### Format

```json
"called_services": [
  {
    "service": "payment",
    "endpoint": "charge",
    "traffic_forward_ratio": 1,
    "resilience_patterns": {
      ...
    }
  }
]
```

### Exponential Backoff

The exponential backoff pattern automatically retries failed requests while increasing the waiting time between attempts. The delay is calculated using the configured multiplier until the maximum delay or maximum number of attempts is reached.

#### Required attributes

* **initial**: Determines the initial delay between retry attempts in seconds.
* **max**: Determines the maximum delay allowed between retry attempts in seconds.
* **multiplier**: Determines the factor used to increase the delay after each retry.
* **max_attempts**: Determines the maximum number of retry attempts before giving up.

#### Format

```json
"resilience_patterns": {
  "exponential_backoff": {
    "initial": <float>,
    "max": <float>,
    "multiplier": <float>,
    "max_attempts": <integer>
  }
}
```

#### Example

```json
"resilience_patterns": {
  "exponential_backoff": {
    "initial": 0.5,
    "max": 10,
    "multiplier": 2,
    "max_attempts": 5
  }
}
```

### Timeout

The timeout pattern limits the maximum amount of time that a downstream request is allowed to execute. If the configured duration is exceeded, the request is aborted and considered failed.

#### Required attributes

* **duration**: Determines the maximum request duration in seconds.

#### Format

```json
"resilience_patterns": {
  "timeout": {
    "duration": <float>
  }
}
```

#### Example

```json
"resilience_patterns": {
  "timeout": {
    "duration": 2.5
  }
}
```

### Fallback

The fallback pattern defines an alternative action when a downstream request cannot be completed successfully.

Two fallback modes are currently supported:

- `static`: Returns a predefined response.
- `service`: Redirects the request to an alternative service endpoint.

#### Required attributes

* **type**: Determines the fallback strategy (`static` or `service`).

#### Static fallback attributes

* **response_code**: The HTTP status code to return.
* **response_payload**: The response payload returned to the caller.

#### Service fallback attributes

* **service**: The name of the fallback service.
* **endpoint**: The endpoint exposed by the fallback service.
* **port**: The port used by the fallback service.

#### Static Fallback Example

```json
"resilience_patterns": {
  "fallback": {
    "type": "static",
    "response_code": 200,
    "response_payload": "{\"status\":\"fallback\"}"
  }
}
```

#### Service Fallback Example

```json
"resilience_patterns": {
  "fallback": {
    "type": "service",
    "service": "backup-payment",
    "endpoint": "charge",
    "port": 8080
  }
}
```


## Describing Resource Stressors

HydraGen supports parameters to express the computational complexity or stress a microservice exerts on the different hardware resources. Initially, CPU-bounded or network-bounded tasks are implemented. The complexity of a CPU-bounded task can be described based on the time a busy-wait is executed, while the load on the network I/O can be described by specifying parameters such as the call forwarding mode and the request/response size for each service endpoint call.

Documentation for implementing a new stressor can be found [here](stressors.md).

### CPU Complexity

The CPU stressor will lock threads for exclusive access while it is executing. This prevents the service from responding to requests on that thread.

#### Required attributes

* **execution_time**: Determines how much time each thread will spend busy-waiting when responding to a request.

#### Optional attributes

* **threads**: The number of threads (goroutines) the CPU stressor should execute the busy-wait loop on. Default: 1

#### Format

```json
"cpu_complexity": {
  "execution_time": <float:seconds>,
  "threads": <integer>
}
```

### Network Complexity

#### Optional attributes

* **forward_requests**: Determines if several calls to endpoints should be made in parallel. Default: "synchronous"
* **response_payload_size**: Determines the number of characters that the server should send back to the calling service or stressor. Default: 0
* **called_services**: An array of endpoints that this endpoint will call before responding. Default: empty array

```json
"network_complexity": {
  "forward_requests": "<string:synchronous|asynchronous>",
  "response_payload_size": <integer:chars>,
  "called_services": [...]
}
```

### Endpoint Calls

#### Required attributes

* **service**: The name of the service object that serves the specified endpoint.
* **endpoint**: The name of the endpoint that will be contacted.
* **traffic_forward_ratio**: Determines the ratio of inbound to outbound requests (1:X). This determines how many times an endpoint call will be made for every request.

#### Optional attributes

* **request_payload_size**: Determines the number of characters that will be sent in the request to the endpoint. Default: 0
* **port**: The port the server is responding to requests on. This is usually determined automatically.
* **protocol**: Determines if the call will be made using HTTP or gRPC. This is usually determined automatically.
* **active_circuit_breaker**: Determines if the endpoint circuit breaker will protect this service call.

#### Format

```json
"called_services": [
  {
    "service": "<string>",
    "endpoint": "<string>",
    "port": "<string>",
    "protocol": "<string:http|grpc>",
    "traffic_forward_ratio": <integer>,
    "request_payload_size": <integer:chars>,
    "active_circuit_breaker": <boolean>
  }
]
```

# Examples

Examples for simple and complex applications generated with HydraGen can be found [here](https://github.com/EricssonResearch/cloud-native-app-simulator/tree/main/generator/examples). The .json is the taxonomy description give as input to the application generator and the the clusterX folder(s) contain the Kubernetes .yaml files generated by this module.

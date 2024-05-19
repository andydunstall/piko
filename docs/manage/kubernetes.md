# Kubernetes

Piko is designed to be easy to host as a cluster of nodes on Kubernetes. When
Piko is hosted as a cluster it will gracefully handle nodes leaving and
joining.

## Discovery

To deploy Piko as a cluster, configure `--cluster.join` to a list of cluster
members in the cluster to join.

The addresses may be either addresses of specific nodes, such as
`10.26.104.14`, or a domain name that resolves to the IP addresses of all nodes
in the cluster.

On Kubernetes, its best to create a headless service for Piko that resolves to
the IP addresses of each Piko pod in the cluster.

You can then configure `--cluster.join` with the service name, `piko`.

## Example

This example creates a Piko cluster with 3 replicas using a StatefulSet.

First create a headless service which is used for cluster discovery:
```
apiVersion: v1
kind: Service
metadata:
  name: piko
  labels:
    app: piko
spec:
  ports:
  - port: 8000
    name: proxy
  - port: 8001
    name: upstream
  - port: 8002
    name: admin
  - port: 8003
    name: gossip
  clusterIP: None
  selector:
    app: piko
```

Next create a YAML config map. To make debugging easier, this uses the pod name
as the Piko node ID prefix. Note the pod name must not be used as the node ID
itself since each restart requires a new node ID. This also configures cluster
discovery to use the service created above:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: server-config
data:
  server.yaml: |
    cluster:
      node_id_prefix: ${POD_NAME}-
      join:
        - piko
```

Finally create a StatefulSet with three replicas:

```
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: piko
spec:
  selector:
    matchLabels:
      app: piko
  serviceName: "piko"
  replicas: 3
  template:
    metadata:
      labels:
        app: piko
    spec:
      terminationGracePeriodSeconds: 60
      containers:
      - name: piko
        image: my-repo/piko:latest
        ports:
        - containerPort: 8000
          name: proxy
        - containerPort: 8001
          name: upstream
        - containerPort: 8002
          name: admin
        - containerPort: 8003
          name: gossip
        args:
          - server
          - --config.path
          - /config/server.yaml
          - --config.expand-env
       resources:
          limits:
            cpu: 250m
            ephemeral-storage: 1Gi
            memory: 1Gi
          requests:
            cpu: 250m
            ephemeral-storage: 1Gi
            memory: 1Gi
        env:
          - name: POD_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
        volumeMounts:
          - name: config
            mountPath: "/config"
            readOnly: true
      volumes:
        - name: config
          configMap:
            name: server-config
            items:
            - key: "server.yaml"
              path: "server.yaml"
```

You can then setup the any required load balancers (such as a Kubernetes
Gatweay) or services to route requests to the server.
to Piko. 

## Helm

Piko includes a simple helm chart at [operations/helm](../../operations/helm).
This chart creates a headless service and StatefulSet as described above.

# Volume Initializer

# Introduction
This project delivers a [mutating admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) that can be used to initialize the pvc volumes of pod by injecting init containers into the pod.

The pvc volumes will be mounted to the injected init containers, you can do anything you want to the volumes, such as changing the ownership/permissions/contents of the volumes, just before your original container starts.

One typical usecase is using it to change the ownership/permissions of the volumes because your original containers are not running as root and unable to write data into the volumes.

# Installation 

## Deploy CRD
```sh
kubectl apply -f config/crd/bases
```

## Deploy CR
Create a volume initializer yaml and apply it.

Take [this](config/samples/storage.kubesphere.io_v1alpha1_initializer.yaml) for example.

## Deploy Webhook

```sh
deploy/prepare.sh && kubectl apply -f deploy/webhook-deployment.yaml
```

## Test 
Create pod with pvc volumes to test.

Take [this](config/samples/mongo-test.yaml) for example. This example requires you have storage class named `local-path` and `local-path2` on your cluster. You can install the [local-path-provisioner](https://github.com/rancher/local-path-provisioner) for quick testing.

# Environment Variables
The following environment variables will be present in the injected init container.

| Environment Variable | Explanation                                                                                                                                     | Present When      | Example Values    |
|----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|-------------------|-------------------|
| PVC_1_MOUNT_PATH     | pvc volume's mount path in the init container                                                                                                   | Always            | `/data`           |
| PVC_1_UID            | value from pod's label `volume.storage.kubesphere.io/uid` or `${volume-name}.volume.storage.kubesphere.io/uid`, can be used to chown the volume | When label exists | `mongodb`, `1001` |
| PVC_1_GID            | value from pod's label `volume.storage.kubesphere.io/gid` or `${volume-name}.volume.storage.kubesphere.io/gid`, can be used to chown the volume | When label exists | `0`, `mongodb`    |


# FAQ
1. Why not use pod's annotations instead of labels to pass the volume's UID/GID to init container?
- The webhook listens the pod CREATE events, such pods are likely generated from replicaset(from deployment/statefulset/daemonset), and normally don't have annotations present at the admission stage (i.e. when this webhook processes the requests). Therefore, we need to use the labels.


# Limitations
- If the pvc matches multiple pvcMatchers and init containers, only the first init container will be injected.


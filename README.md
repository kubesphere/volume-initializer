# Volume Initializer

# Introduction
This project delivers a [mutating admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) that can be used to initialize the pvc volumes of pod by injecting init containers into the pod.

The pvc volumes will be mounted to the injected init containers, you can do anything you want to the volumes, such as changing the ownership/permissions/contents of the volumes, just before your original container starts.

One typical usecase is using it to change the ownership/permissions of the volumes because your original containers are not running as root and unable to write data into the volumes.

# Installation 

## Deploy CRD
```
kubectl apply -f config/crd/bases
```

## Deploy CR
Create a volume initializer yaml and apply it.

Take [this](config/samples/storage.kubesphere.io_v1alpha1_initializer.yaml) for example.

## Deploy webhook
```
kubectl apply -f deploy/webhook-deployment.yaml
```

## Test 
Create pod with pvc volumes to test.

Take [this](config/samples/mongo-test.yaml) for example.

# Environment Variables
The following environment variables will be present in the injected init container.

| Environment Variable | Explanation                                                                                                                                          | Present When           | Example Values    |
|----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------|-------------------|
| PVC_1_MOUNT_PATH     | pvc volume's mount path in the init container                                                                                                        | Always                 | `/data`           |
| PVC_1_UID            | value from pod's annotation `volume.storage.kubesphere.io/uid` or `${volume-name}.volume.storage.kubesphere.io/uid`, can be used to chown the volume | When annotation exists | `mongodb`, `1001` |
| PVC_1_GID            | value from pod's annotation `volume.storage.kubesphere.io/gid` or `${volume-name}.volume.storage.kubesphere.io/gid`, can be used to chown the volume | When annotation exists | `0`, `mongodb`    |


# Limitations
- If the pvc matches multiple pvcMatchers and init containers, only the first init container will be injected.


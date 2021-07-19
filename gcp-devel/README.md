# GCP/GKE Dev Environment

This Pulumi stack creates a development environment in GCP.

## Configuration

| Key | Default | Description |
| --- | --- | ---|
| gcp-devel:clusters.kubernetes.channel              | `"regular"` | Release channel for Kubernetes |
| gcp-devel:clusters.kubernetes.version              | `"1.20.6-gke.1000"` | Kubernetes version |
| gcp-devel:clusters.names              | `["global", "zone-1", "zone-2" ]` | Names of the clusters to create (by providing `n` names it will create `n` clusters) |
| gcp-devel:clusters.networkPolicy              | `true` | If enable network policy addon (which also uses CALICO instead of native GCP networking plugin) |
| gcp-devel:clusters.nodeConfig.machineType              | `"n1-standard-2"` | The type of worker nodes to use in a cluster |
| gcp-devel:clusters.nodeConfig.oauthScopes              | `[ "https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/devstorage.read_only", "https://www.googleapis.com/auth/logging.write", "https://www.googleapis.com/auth/monitoring", "https://www.googleapis.com/auth/servicecontrol", "https://www.googleapis.com/auth/service.management.readonly", "https://www.googleapis.com/auth/trace.append" ]` | OAuth scopes for worker nodes |
| gcp-devel:clusters.nodeConfig.preemptible              | `true` | Should the worker nodes be preemptible |
| gcp-devel:clusters.nodeConfig.nodeLocations              | `["us-central1-c"]` | Locations of worker nodes |
| gcp-devel:location              | `"us-central1"` | Location |
| gcp-devel:resourcePrefix              | `"kuma"` | Name prefix for the all resources |
| gcp:project              | | Name of the GCP project under which resources will be created |

Use [pulumi config](https://www.pulumi.com/docs/intro/concepts/config/)
to change the configuration.

## Sample session

```
$ pulumi up
Previewing update (dev):
     Type                                Name                               Plan       
 +   pulumi:pulumi:Stack                 gcp-devel-dev                      create     
 +   ├─ gcp:serviceAccount:Account       kuma-bartsmykla                    create     
 +   ├─ gcp:compute:Network              kuma-bartsmykla-network            create     
 +   │  ├─ gcp:compute:Subnetwork        kuma-bartsmykla-subnet             create     
 +   │  └─ gcp:compute:Firewall          kuma-bartsmykla-allow-ssh          create     
 +   ├─ gcp:secretmanager:Secret         kuma-bartsmykla-global-kubeconfig  create     
 +   ├─ gcp:secretmanager:Secret         kuma-bartsmykla-zone-1-kubeconfig  create     
 +   ├─ gcp:secretmanager:Secret         kuma-bartsmykla-zone-2-kubeconfig  create     
 +   ├─ gcp:container:Cluster            kuma-bartsmykla-zone-1             create     
 +   ├─ gcp:container:Cluster            kuma-bartsmykla-zone-2             create     
 +   ├─ gcp:container:Cluster            kuma-bartsmykla-global             create     
 +   ├─ gcp:secretmanager:SecretVersion  kuma-bartsmykla-zone-1-kubeconfig  create     
 +   ├─ gcp:secretmanager:SecretVersion  kuma-bartsmykla-zone-2-kubeconfig  create     
 +   └─ gcp:secretmanager:SecretVersion  kuma-bartsmykla-global-kubeconfig  create     
 
Resources:
    + 14 to create

Do you want to perform this update? yes
Updating (dev):
     Type                                Name                               Status      
 +   pulumi:pulumi:Stack                 gcp-devel-dev                      created     
 +   ├─ gcp:compute:Network              kuma-bartsmykla-network            created     
 +   │  ├─ gcp:compute:Firewall          kuma-bartsmykla-allow-ssh          created     
 +   │  └─ gcp:compute:Subnetwork        kuma-bartsmykla-subnet             created     
 +   ├─ gcp:serviceAccount:Account       kuma-bartsmykla                    created     
 +   ├─ gcp:secretmanager:Secret         kuma-bartsmykla-global-kubeconfig  created     
 +   ├─ gcp:secretmanager:Secret         kuma-bartsmykla-zone-1-kubeconfig  created     
 +   ├─ gcp:secretmanager:Secret         kuma-bartsmykla-zone-2-kubeconfig  created     
 +   ├─ gcp:container:Cluster            kuma-bartsmykla-zone-2             created     
 +   ├─ gcp:container:Cluster            kuma-bartsmykla-zone-1             created     
 +   ├─ gcp:container:Cluster            kuma-bartsmykla-global             created     
 +   ├─ gcp:secretmanager:SecretVersion  kuma-bartsmykla-zone-2-kubeconfig  created     
 +   ├─ gcp:secretmanager:SecretVersion  kuma-bartsmykla-global-kubeconfig  created     
 +   └─ gcp:secretmanager:SecretVersion  kuma-bartsmykla-zone-1-kubeconfig  created     
 
Outputs:
    kuma-bartsmykla-global-kubeconfig: "[secret]"
    kuma-bartsmykla-zone-1-kubeconfig: "[secret]"
    kuma-bartsmykla-zone-2-kubeconfig: "[secret]"
    private-key                      : "[secret]"
    public-key                       : "[secret]"
    service-account                  : "kuma-bartsmykla"

Resources:
    + 14 created

Duration: 4m33s
```

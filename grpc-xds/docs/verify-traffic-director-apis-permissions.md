# Verify APIs and IAM permissions for Traffic Director and GKE

This document describes steps that you can take to verify that you can set up
a GKE cluster and a Traffic Director service mesh in a Google Cloud
[project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).

You run commands to verify that billing is setup, the necessary Google Cloud
APIs are enabled, and that you have the necessary IAM
[permissions](https://cloud.google.com/iam/docs/overview#permissions) and
[roles](https://cloud.google.com/iam/docs/overview#roles) to create the
required resources in the project.

## Before you begin

1.  Install the
    [Google Cloud SDK](https://cloud.google.com/sdk/docs/install).

2.  Authorize the `gcloud` command-line tool to access Google Cloud APIs with
    your
    [Google Account](https://cloud.google.com/iam/docs/overview#google-account)
    credentials:

    ```shell
    gcloud auth login
    ```

3.  Create an environment variable with the
    [project ID](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
    of the Google Cloud project you will be using:

    ```shell
    export PROJECT_ID=my_project_id
    ```

    Replace `my_project_id` with your project ID.

4.  Configure `gcloud` to use the Google Cloud project as the default:

    ```shell
    gcloud config set project $PROJECT_ID
    ```

    This setting means that you don't need to add the `--project` flag every
    time you use the `gcloud` tool.

5.  Install `kubectl` and authentication plugins:

    ```shell
    gcloud components install gke-gcloud-auth-plugin kubectl kubectl-oidc
    ```

## Billing

1.  Verify that billing is enabled on your project:

    ```shell
    gcloud billing projects describe $PROJECT_ID
    ```

    Billing is enabled if the output includes this line:

    ```yaml
    billingEnabled: true
    ```

    You need the following on the billing account to confirm that billing is
    enabled on your project:

    - [`billing.resourceAssociations.list`](https://cloud.google.com/billing/docs/how-to/custom-roles#resource_associations)
    - [`billing.accounts.get`](https://cloud.google.com/billing/docs/how-to/custom-roles#account_management)

    [Billing Account Viewer (`roles/billing.viewer`)](https://cloud.google.com/iam/docs/understanding-roles#billing-roles)
    is an IAM role that grants these permissions. Other Cloud Billing roles
    are available, for details see
    [Cloud Billing access control & permissions](https://cloud.google.com/billing/docs/how-to/billing-access).

## Google Cloud APIs

1.  Verify that the necessary Google Cloud APIs (or "services") are enabled on
    the project:

    ```shell
    gcloud services list --enabled
    ```

    The output should include at least the following APIs:

    ```
    artifactregistry.googleapis.com      Artifact Registry API
    autoscaling.googleapis.com           Cloud Autoscaling API
    cloudresourcemanager.googleapis.com  Cloud Resource Manager API
    compute.googleapis.com               Compute Engine API
    container.googleapis.com             Kubernetes Engine API
    containerfilesystem.googleapis.com   Container File System API
    containerregistry.googleapis.com     Container Registry API
    dns.googleapis.com                   Cloud DNS API
    iam.googleapis.com                   Identity and Access Management (IAM) API
    iamcredentials.googleapis.com        IAM Service Account Credentials API
    logging.googleapis.com               Cloud Logging API
    monitoring.googleapis.com            Cloud Monitoring API
    networkconnectivity.googleapis.com   Network Connectivity API
    networksecurity.googleapis.com       Network Security API
    networkservices.googleapis.com       Network Services API
    osconfig.googleapis.com              OS Config API
    oslogin.googleapis.com               Cloud OS Login API
    privateca.googleapis.com             Certificate Authority API
    serviceusage.googleapis.com          Service Usage API
    storage-api.googleapis.com           Google Cloud Storage JSON API
    trafficdirector.googleapis.com       Traffic Director API
    ```

    You need the permissions of the
    [Service Usage Viewer role (`roles/serviceusage.serviceUsageViewer`)](https://cloud.google.com/iam/docs/understanding-roles#service-usage-roles)
    on the project to list the enabled APIs.

    If one or more of these APIs have not been enabled, you can enable them:

    ```shell
    gcloud services enable API.googleapis.com
    ```

    Replace `API` with the name of the API you want to enable, e.g.,
    `container` to enable the Kubernetes Engine API.

    You need the permissions of the
    [Service Usage Admin role (`roles/serviceusage.serviceUsageAdmin`)](https://cloud.google.com/iam/docs/understanding-roles#service-usage-roles)
    on the project to enable APIs.

## IAM roles and permissions

You must have sufficient Identity and Access Management (IAM) permissions on
the project to create the resources necessary for Traffic Director and GKE.

If you have the role of project
[Owner or Editor](https://cloud.google.com/iam/docs/understanding-roles#basic)
(`roles/owner` or `roles/editor`) on the project, you have the necessary
permissions. Otherwise, see below for roles that provide the necessary
permissions.

1.  List the IAM roles that have been granted to your Google Account on the
    project:

    ```shell
    gcloud projects get-iam-policy $PROJECT_ID \
      --flatten bindings \
      --format yaml \
      --filter "bindings.members=user:$(gcloud config get account)"
    ```

    The output shows IAM policy bindings, i.e., IAM roles granted, on the
    project to your Google Account.

    E.g., if your Google Account `user@example.com` has been granted the
    Project Billing Manager and Editor roles on the project, the output looks
    similar to this:

    ```yaml
    bindings:
      members:
      - user:user@example.com
      role: roles/billing.projectManager
    ---
    bindings:
      members:
      - user:user@example.com
      role: roles/editor
    etag: BwYf_sAhAjI=
    version: 1
    ```

    To create a VPC network and add firewall rules, you need the following
    roles:

    - [Compute Network Admin (`roles/compute.networkAdmin`)](https://cloud.google.com/compute/docs/access/iam#compute.networkAdmin)
      to create VPC networks and subnets, Cloud Routers, NAT gateways, and
      [Service Routing API resources](https://cloud.google.com/service-mesh/docs/service-routing/service-routing-overview)
      for Traffic Director.

    - [Compute Security Admin (`roles/compute.securityAdmin`)](https://cloud.google.com/compute/docs/access/iam#compute.securityAdmin)
      to add and remove firewall rules.

    To create a GKE cluster, you need the following roles:

    - [Compute Instance Admin (`roles/compute.instanceAdmin`)](https://cloud.google.com/compute/docs/access/iam#compute.instanceAdmin)
      to create Compute Engine VM instances.

    - [Kubernetes Engine Admin (`roles/container.admin`)](https://cloud.google.com/iam/docs/understanding-roles#container.admin)
      to create and access GKE clusters and their Kubernetes API objects.

    - [Service Account Admin (`roles/iam.serviceAccountAdmin`)](https://cloud.google.com/iam/docs/understanding-roles#iam.serviceAccountAdmin)
      to create and set IAM policies for IAM service accounts. You use this to
      create an IAM service account with minimum permissions required for GKE
      cluster nodes.
      
      Alternatively, the GKE cluster nodes can use the
      [Compute Engine default service account](https://cloud.google.com/compute/docs/access/service-accounts#default_service_account).
      This IAM service account is created when you enable the Compute Engine
      API, and it is the IAM service account assigned to the GKE cluster nodes
      by default.

    - [Service Account User (`roles/iam.serviceAccountUser`)](https://cloud.google.com/iam/docs/understanding-roles#iam.serviceAccountUser)
      either on the project, or on the IAM service account that you assign
      to GKE cluster nodes.

    To set up Traffic Director, you need the following roles:

    - [Service Usage Admin (`roles/serviceusage.serviceUsageAdmin`)](https://cloud.google.com/iam/docs/understanding-roles#service-usage-roles)
      is required to enable the `trafficdirector.googleapis.com` API.

    - [Service Account User (`roles/iam.serviceAccountUser`)](https://cloud.google.com/iam/docs/understanding-roles#iam.serviceAccountUser)
      either on the project, or on the IAM service account that you assign
      to workloads in the GKE cluster.

    - [Traffic Director Client (`roles/trafficdirector.client`)](https://cloud.google.com/iam/docs/understanding-roles#trafficdirector.client)
      is required so that you can assign that role to the IAM service accounts
      that you create for the GKE cluster workloads. The role provides access
      to fetch xDS resources from the Traffic Director xDS control plane.

    To set up mTLS and workload authorization with
    [Certificate Authority Service](https://cloud.google.com/certificate-authority-service/docs/ca-service-overview),
    (CA Service), you need the following role:

    - [CA Service Admin (`roles/privateca.admin`)](https://cloud.google.com/iam/docs/understanding-roles#privateca.admin)
      to create CAs for issuing SPIFFE SVID X.509 certificates to workloads
      in the service mesh.

    If you are missing some of the required IAM policy bindings on the
    project, you can add them:

    ```shell
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member user:$(gcloud config get account) \
      --role ROLE
    ```

    Replace `ROLE` with the role you want to grant, e.g.,
    `roles/compute.instanceAdmin` for the Compute Instance Admin role.

    You need the permissions of the
    [Project IAM Admin role (`roles/resourcemanager.projectIamAdmin`)](https://cloud.google.com/iam/docs/understanding-roles#resourcemanager.projectIamAdmin)
    on the project to grant roles on the project.

    If you want to see how a change to an IAM policy binding might impact
    access before making the change, you can use the
    [IAM Policy Simulator](https://cloud.google.com/policy-intelligence/docs/iam-simulator-overview).

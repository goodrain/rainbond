package cluster

var versions = map[string]Version{
	"5.3.1": {
		CRDs: []string{
			componentdefinitionsCRD531,
			helmappCRD531,
			thirdcomponentCRD531,
		},
	},
}

type Version struct {
	CRDs []string
}

var componentdefinitionsCRD531 = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.3.0
  creationTimestamp: null
  name: componentdefinitions.rainbond.io
spec:
  additionalPrinterColumns:
  - JSONPath: .spec.workload.definition.kind
    name: WORKLOAD-KIND
    type: string
  - JSONPath: .metadata.annotations.definition\.oam\.dev/description
    name: DESCRIPTION
    type: string
  group: rainbond.io
  names:
    categories:
    - oam
    kind: ComponentDefinition
    listKind: ComponentDefinitionList
    plural: componentdefinitions
    shortNames:
    - comp
    singular: componentdefinition
  scope: Namespaced
  subresources:
    status: {}
  validation:
    openAPIV3Schema:
      description: ComponentDefinition is the Schema for the componentdefinitions
        API
      properties:
        apiVersion:
          description: 'APIVersion defines the versioned schema of this representation
            of an object. Servers should convert recognized schemas to the latest
            internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
          type: string
        kind:
          description: 'Kind is a string value representing the REST resource this
            object represents. Servers may infer this from the endpoint the client
            submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
          type: string
        metadata:
          type: object
        spec:
          description: ComponentDefinitionSpec defines the desired state of ComponentDefinition
          properties:
            childResourceKinds:
              description: ChildResourceKinds are the list of GVK of the child resources
                this workload generates
              items:
                description: A ChildResourceKind defines a child Kubernetes resource
                  kind with a selector
                properties:
                  apiVersion:
                    description: APIVersion of the child resource
                    type: string
                  kind:
                    description: Kind of the child resource
                    type: string
                  selector:
                    additionalProperties:
                      type: string
                    description: Selector to select the child resources that the workload
                      wants to expose to traits
                    type: object
                required:
                - apiVersion
                - kind
                type: object
              type: array
            extension:
              description: Extension is used for extension needs by OAM platform builders
              type: object
              x-kubernetes-preserve-unknown-fields: true
            podSpecPath:
              description: PodSpecPath indicates where/if this workload has K8s podSpec
                field if one workload has podSpec, trait can do lot's of assumption
                such as port, env, volume fields.
              type: string
            revisionLabel:
              description: RevisionLabel indicates which label for underlying resources(e.g.
                pods) of this workload can be used by trait to create resource selectors(e.g.
                label selector for pods).
              type: string
            schematic:
              description: Schematic defines the data format and template of the encapsulation
                of the workload
              properties:
                cue:
                  description: CUE defines the encapsulation in CUE format
                  properties:
                    template:
                      description: Template defines the abstraction template data
                        of the capability, it will replace the old CUE template in
                        extension field. Template is a required field if CUE is defined
                        in Capability Definition.
                      type: string
                  required:
                  - template
                  type: object
              type: object
            status:
              description: Status defines the custom health policy and status message
                for workload
              properties:
                customStatus:
                  description: CustomStatus defines the custom status message that
                    could display to user
                  type: string
                healthPolicy:
                  description: HealthPolicy defines the health check policy for the
                    abstraction
                  type: string
              type: object
            workload:
              description: Workload is a workload type descriptor
              properties:
                definition:
                  description: Definition mutually exclusive to workload.type, a embedded
                    WorkloadDefinition
                  properties:
                    apiVersion:
                      type: string
                    kind:
                      type: string
                  required:
                  - apiVersion
                  - kind
                  type: object
                type:
                  description: Type ref to a WorkloadDefinition via name
                  type: string
              type: object
          required:
          - workload
          type: object
        status:
          description: ComponentDefinitionStatus is the status of ComponentDefinition
          properties:
            conditions:
              description: Conditions of the resource.
              items:
                description: A Condition that may apply to a resource.
                properties:
                  lastTransitionTime:
                    description: LastTransitionTime is the last time this condition
                      transitioned from one status to another.
                    format: date-time
                    type: string
                  message:
                    description: A Message containing details about this condition's
                      last transition from one status to another, if any.
                    type: string
                  reason:
                    description: A Reason for this condition's last transition from
                      one status to another.
                    type: string
                  status:
                    description: Status of this condition; is it currently True, False,
                      or Unknown?
                    type: string
                  type:
                    description: Type of this condition. At most one of each condition
                      type may apply to a resource at any point in time.
                    type: string
                required:
                - lastTransitionTime
                - reason
                - status
                - type
                type: object
              type: array
            configMapRef:
              description: ConfigMapRef refer to a ConfigMap which contains OpenAPI
                V3 JSON schema of Component parameters.
              type: string
            latestRevision:
              description: LatestRevision of the component definition
              properties:
                name:
                  type: string
                revision:
                  format: int64
                  type: integer
                revisionHash:
                  description: RevisionHash record the hash value of the spec of ApplicationRevision
                    object.
                  type: string
              required:
              - name
              - revision
              type: object
          type: object
      type: object
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`

var helmappCRD531 = `
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.3.0
  creationTimestamp: null
  name: helmapps.rainbond.io
spec:
  group: rainbond.io
  names:
    kind: HelmApp
    listKind: HelmAppList
    plural: helmapps
    singular: helmapp
  scope: Namespaced
  subresources:
    status: {}
  validation:
    openAPIV3Schema:
      description: HelmApp -
      properties:
        apiVersion:
          description: 'APIVersion defines the versioned schema of this representation
            of an object. Servers should convert recognized schemas to the latest
            internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
          type: string
        kind:
          description: 'Kind is a string value representing the REST resource this
            object represents. Servers may infer this from the endpoint the client
            submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
          type: string
        metadata:
          type: object
        spec:
          description: HelmAppSpec defines the desired state of HelmApp
          properties:
            appStore:
              description: The helm app store.
              properties:
                branch:
                  description: The branch of a git repo.
                  type: string
                name:
                  description: The name of app store.
                  type: string
                password:
                  description: The chart repository password where to locate the requested
                    chart
                  type: string
                url:
                  description: The url of helm repo, sholud be a helm native repo
                    url or a git url.
                  type: string
                username:
                  description: The chart repository username where to locate the requested
                    chart
                  type: string
                version:
                  description: The verision of the helm app store.
                  type: string
              required:
              - name
              - url
              - version
              type: object
            eid:
              type: string
            overrides:
              description: Overrides will overrides the values in the chart.
              items:
                type: string
              type: array
            preStatus:
              description: The prerequisite status.
              enum:
              - NotConfigured
              - Configured
              type: string
            revision:
              description: The application revision.
              type: integer
            templateName:
              description: The application name.
              type: string
            version:
              description: The application version.
              type: string
          required:
          - appStore
          - eid
          - templateName
          - version
          type: object
        status:
          description: HelmAppStatus defines the observed state of HelmApp
          properties:
            conditions:
              description: Current state of helm app.
              items:
                description: HelmAppCondition contains details for the current condition
                  of this helm application.
                properties:
                  lastTransitionTime:
                    description: Last time the condition transitioned from one status
                      to another.
                    format: date-time
                    type: string
                  message:
                    description: Human-readable message indicating details about last
                      transition.
                    type: string
                  reason:
                    description: Unique, one-word, CamelCase reason for the condition's
                      last transition.
                    type: string
                  status:
                    description: 'Status is the status of the condition. Can be True,
                      False, Unknown. More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions'
                    type: string
                  type:
                    description: Type is the type of the condition.
                    type: string
                required:
                - status
                - type
                type: object
              type: array
            currentVersion:
              description: The version infect.
              type: string
            overrides:
              description: Overrides in effect.
              items:
                type: string
              type: array
            phase:
              description: The phase of the helm app.
              type: string
            status:
              description: The status of helm app.
              type: string
          required:
          - phase
          - status
          type: object
      type: object
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []

`

var thirdcomponentCRD531 = `

---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.3.0
  creationTimestamp: null
  name: thirdcomponents.rainbond.io
spec:
  group: rainbond.io
  names:
    kind: ThirdComponent
    listKind: ThirdComponentList
    plural: thirdcomponents
    singular: thirdcomponent
  scope: Namespaced
  subresources:
    status: {}
  validation:
    openAPIV3Schema:
      description: HelmApp -
      properties:
        apiVersion:
          description: 'APIVersion defines the versioned schema of this representation
            of an object. Servers should convert recognized schemas to the latest
            internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
          type: string
        kind:
          description: 'Kind is a string value representing the REST resource this
            object represents. Servers may infer this from the endpoint the client
            submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
          type: string
        metadata:
          type: object
        spec:
          properties:
            endpointSource:
              description: endpoint source config
              properties:
                endpoints:
                  items:
                    properties:
                      address:
                        description: The address including the port number.
                        type: string
                      clientSecret:
                        description: Specify a private certificate when the protocol
                          is HTTPS
                        type: string
                      protocol:
                        description: 'Address protocols, including: HTTP, TCP, UDP,
                          HTTPS'
                        type: string
                    required:
                    - address
                    type: object
                  type: array
                kubernetesService:
                  properties:
                    name:
                      type: string
                    namespace:
                      description: If not specified, the namespace is the namespace
                        of the current resource
                      type: string
                  required:
                  - name
                  type: object
              type: object
            ports:
              description: component regist ports
              items:
                description: ComponentPort component port define
                properties:
                  name:
                    type: string
                  openInner:
                    type: boolean
                  openOuter:
                    type: boolean
                  port:
                    type: integer
                required:
                - name
                - openInner
                - openOuter
                - port
                type: object
              type: array
            probe:
              description: health check probe
              properties:
                httpGet:
                  description: HTTPGet specifies the http request to perform.
                  properties:
                    httpHeaders:
                      description: Custom headers to set in the request. HTTP allows
                        repeated headers.
                      items:
                        description: HTTPHeader describes a custom header to be used
                          in HTTP probes
                        properties:
                          name:
                            description: The header field name
                            type: string
                          value:
                            description: The header field value
                            type: string
                        required:
                        - name
                        - value
                        type: object
                      type: array
                    path:
                      description: Path to access on the HTTP server.
                      type: string
                  type: object
                tcpSocket:
                  description: 'TCPSocket specifies an action involving a TCP port.
                    TCP hooks not yet supported TODO: implement a realistic TCP lifecycle
                    hook'
                  type: object
              type: object
          required:
          - endpointSource
          - ports
          type: object
        status:
          properties:
            endpoints:
              items:
                description: ThirdComponentEndpointStatus endpoint status
                properties:
                  address:
                    description: The address including the port number.
                    type: string
                  reason:
                    description: Reason probe not passed reason
                    type: string
                  servicePort:
                    description: ServicePort if address build from kubernetes endpoint,
                      The corresponding service port
                    type: integer
                  status:
                    description: Status endpoint status
                    type: string
                  targetRef:
                    description: Reference to object providing the endpoint.
                    properties:
                      apiVersion:
                        description: API version of the referent.
                        type: string
                      fieldPath:
                        description: 'If referring to a piece of an object instead
                          of an entire object, this string should contain a valid
                          JSON/Go field access statement, such as desiredState.manifest.containers[2].
                          For example, if the object reference is to a container within
                          a pod, this would take on a value like: "spec.containers{name}"
                          (where "name" refers to the name of the container that triggered
                          the event) or if no container name is specified "spec.containers[2]"
                          (container with index 2 in this pod). This syntax is chosen
                          only to have some well-defined way of referencing a part
                          of an object. TODO: this design is not final and this field
                          is subject to change in the future.'
                        type: string
                      kind:
                        description: 'Kind of the referent. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
                        type: string
                      name:
                        description: 'Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names'
                        type: string
                      namespace:
                        description: 'Namespace of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/'
                        type: string
                      resourceVersion:
                        description: 'Specific resourceVersion to which this reference
                          is made, if any. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency'
                        type: string
                      uid:
                        description: 'UID of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids'
                        type: string
                    type: object
                required:
                - address
                - status
                type: object
              type: array
            phase:
              type: string
            reason:
              type: string
          required:
          - endpoints
          - phase
          type: object
      type: object
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`

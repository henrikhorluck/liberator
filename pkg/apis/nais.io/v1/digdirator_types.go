package nais_io_v1

import (
	"github.com/nais/liberator/pkg/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DigdiratorStatus defines the observed state of Current Client
type DigdiratorStatus struct {
	// SynchronizationState denotes the last known state of the Instance during synchronization
	SynchronizationState string `json:"synchronizationState,omitempty"`
	// SynchronizationTime is the last time the Status subresource was updated
	SynchronizationTime *metav1.Time `json:"synchronizationTime,omitempty"`
	// SynchronizationHash is the hash of the Instance object
	SynchronizationHash string `json:"synchronizationHash,omitempty"`
	// SynchronizationSecretName is the SecretName set in the last successful synchronization
	SynchronizationSecretName string `json:"synchronizationSecretName,omitempty"`
	// ClientID is the corresponding client ID for this client at Digdir
	ClientID string `json:"clientID,omitempty"`
	// CorrelationID is the ID referencing the processing transaction last performed on this resource
	CorrelationID string `json:"correlationID,omitempty"`
	// KeyIDs is the list of key IDs for valid JWKs registered for the client at Digdir
	KeyIDs []string `json:"keyIDs,omitempty"`
	// ApplicationScope is Unique Scopes activated and registered for this application at digdir
	ApplicationScope ApplicationScope `json:"applicationScopes,omitempty"`
}

func (in *DigdiratorStatus) GetSynchronizationHash() string {
	return in.SynchronizationHash
}

func (in *DigdiratorStatus) SetHash(hash string) {
	in.SynchronizationHash = hash
}

func (in *DigdiratorStatus) SetStateSynchronized() {
	now := metav1.Now()
	in.SynchronizationTime = &now
	in.SynchronizationState = EventSynchronized
}

func (in *DigdiratorStatus) GetClientID() string {
	return in.ClientID
}

func (in *DigdiratorStatus) SetClientID(clientID string) {
	in.ClientID = clientID
}

func (in *DigdiratorStatus) SetCorrelationID(correlationID string) {
	in.CorrelationID = correlationID
}

func (in *DigdiratorStatus) GetKeyIDs() []string {
	return in.KeyIDs
}

func (in *DigdiratorStatus) SetKeyIDs(keyIDs []string) {
	in.KeyIDs = keyIDs
}

func (in *DigdiratorStatus) SetSynchronizationState(state string) {
	in.SynchronizationState = state
}

func (in *DigdiratorStatus) GetSynchronizationSecretName() string {
	return in.SynchronizationSecretName
}

func (in *DigdiratorStatus) SetSynchronizationSecretName(name string) {
	in.SynchronizationSecretName = name
}

func (in *DigdiratorStatus) SetApplicationScopeConsumer(applicationScope string, orgNumbers []string) {
	scopes := make(map[string][]string)
	scopes = map[string][]string{
		applicationScope: orgNumbers,
	}
	in.ApplicationScope.Scopes = scopes
}

func (in *DigdiratorStatus) GetApplicationScopes() map[string][]string {
	return in.ApplicationScope.Scopes
}

func init() {
	SchemeBuilder.Register(
		&MaskinportenClient{},
		&MaskinportenClientList{},
	)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=maskinportenclient

// +kubebuilder:printcolumn:name="Secret Ref",type=string,JSONPath=`.spec.secretName`
// +kubebuilder:printcolumn:name="ClientID",type=string,JSONPath=`.status.clientID`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Created",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Synchronized",type="date",JSONPath=".status.synchronizationTime"

// MaskinportenClient is the Schema for the MaskinportenClient API
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MaskinportenClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaskinportenClientSpec `json:"spec,omitempty"`
	Status DigdiratorStatus       `json:"status,omitempty"`
}

// MaskinportenClientSpec defines the desired state of MaskinportenClient
type MaskinportenClientSpec struct {
	// Scopes is a object of used end exposed scopes by application
	Scopes MaskinportenScope `json:"scopes,omitempty"`
	// SecretName is the name of the resulting Secret resource to be created
	SecretName string `json:"secretName"`
}

// MaskinportenClientList contains a list of MaskinportenClient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MaskinportenClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaskinportenClient `json:"items"`
}

// MaskinportenScope is the Schema for the MaskinportenScope API and it contains a list of scopes used
// by an application and scopes exposed by an application
type MaskinportenScope struct {
	UsedScope     []UsedScope    `json:"consumes"`
	ExposedScopes []ExposedScope `json:"exposes,omitempty"`
}

// UsedScope is scope(s) consumed by the application to gain access to external Api(s)
type UsedScope struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// ExposedScope is the exposed scopes exported by the application to grant organization access to resources/apis
type ExposedScope struct {
	// Enabled sets scope availible for use and consumer can be granted access
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`
	// Name is the acutal subscope, build: prefix:<Product><./:>Name
	// +kubebuilder:validation:Pattern=`^([a-zæøå0-9]+\/?)+(\:[a-zæøå0-9]+)*[a-zæøå0-9]+(\.[a-zæøå0-9]+)*$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Product is the product development area an application belongs to. This wil be included in the final scope
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]+$`
	Product string `json:"product"`
	// AtAgeMax Max time in seconds for a issued access_token, defualt is `30`
	// +kubebuilder:validation:Minimum=30
	// +kubebuilder:validation:Maximum=680
	AtAgeMax int `json:"atAgeMax,omitempty"`
	// AllowedIntegrations whitelist of type of integration's allowed. Default is `maskinporten`
	// +kubebuilder:validation:MinItems=1
	AllowedIntegrations []string `json:"allowedIntegrations,omitempty"`
	// Consumers External consumers granted access to this scope and able to get acess_token
	Consumers []ExposedScopeConsumer `json:"consumers,omitempty"`
}

type ExposedScopeConsumer struct {
	// Orgno is the external business (consumer) organisation number
	// +kubebuilder:validation:Pattern=`^\d{9}$`
	Orgno string `json:"orgno"`
	// Name is a describing name intended for clearity.
	Name string `json:"name,omitempty"`
}

type ApplicationScope struct {
	Scopes map[string][]string `json:"scopes,omitempty"`
}

func (in *MaskinportenClient) Hash() (string, error) {
	return hash.Hash(in.Spec)
}

func (in *MaskinportenClient) GetStatus() *DigdiratorStatus {
	return &in.Status
}

func (in *MaskinportenClient) SetStatus(new DigdiratorStatus) {
	in.Status = new
}

func (in MaskinportenClient) GetUsedScopes() []string {
	scopes := make([]string, 0)
	for _, scope := range in.Spec.Scopes.UsedScope {
		scopes = append(scopes, scope.Name)
	}
	return scopes
}

func (in MaskinportenClient) GetExposedScopes() map[string]ExposedScope {
	scopes := make(map[string]ExposedScope)
	for _, scope := range in.Spec.Scopes.ExposedScopes {
		scopes[scope.Name] = scope
	}
	return scopes
}

func init() {
	SchemeBuilder.Register(
		&IDPortenClient{},
		&IDPortenClientList{},
	)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=idportenclient

// +kubebuilder:printcolumn:name="Secret Ref",type=string,JSONPath=`.spec.secretName`
// +kubebuilder:printcolumn:name="ClientID",type=string,JSONPath=`.status.clientID`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Created",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Synchronized",type="date",JSONPath=".status.synchronizationTime"

// IDPortenClient is the Schema for the IDPortenClients API
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IDPortenClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IDPortenClientSpec `json:"spec,omitempty"`
	Status DigdiratorStatus   `json:"status,omitempty"`
}

// IDPortenClientList contains a list of IDPortenClient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IDPortenClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IDPortenClient `json:"items"`
}

// IDPortenClientSpec defines the desired state of IDPortenClient
type IDPortenClientSpec struct {
	// ClientURI is the URL to the client to be used at DigDir when displaying a 'back' button or on errors
	ClientURI string `json:"clientURI,omitempty"`
	// RedirectURI is the redirect URI to be registered at DigDir
	// +kubebuilder:validation:Pattern=`^https:\/\/.+$`
	RedirectURI string `json:"redirectURI"`
	// SecretName is the name of the resulting Secret resource to be created
	SecretName string `json:"secretName"`
	// FrontchannelLogoutURI is the URL that ID-porten sends a requests to whenever a logout is triggered by another application using the same session
	FrontchannelLogoutURI string `json:"frontchannelLogoutURI,omitempty"`
	// PostLogoutRedirectURI is a list of valid URIs that ID-porten may redirect to after logout
	PostLogoutRedirectURIs []string `json:"postLogoutRedirectURIs,omitempty"`
	// SessionLifetime is the maximum session lifetime in seconds for a logged in end-user for this client.
	// +kubebuilder:validation:Minimum=3600
	// +kubebuilder:validation:Maximum=7200
	SessionLifetime *int `json:"sessionLifetime,omitempty"`
	// AccessTokenLifetime is the maximum lifetime in seconds for the returned access_token from ID-porten.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	AccessTokenLifetime *int `json:"accessTokenLifetime,omitempty"`
}

func (in *IDPortenClient) Hash() (string, error) {
	return hash.Hash(in.Spec)
}

func (in *IDPortenClient) GetStatus() *DigdiratorStatus {
	return &in.Status
}

func (in *IDPortenClient) SetStatus(new DigdiratorStatus) {
	in.Status = new
}

/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package fabsdk enables client usage of a Hyperledger Fabric network
package fabsdk

import (
	"github.com/hyperledger/fabric-sdk-go/api/apiconfig"
	"github.com/hyperledger/fabric-sdk-go/api/apicore"
	"github.com/hyperledger/fabric-sdk-go/api/apifabclient"
	"github.com/hyperledger/fabric-sdk-go/api/apilogging"
	"github.com/hyperledger/fabric-sdk-go/api/apitxn"

	"github.com/hyperledger/fabric-sdk-go/api/apicryptosuite"
	"github.com/hyperledger/fabric-sdk-go/def/factory/defclient"
	"github.com/hyperledger/fabric-sdk-go/def/factory/defcore"
	"github.com/hyperledger/fabric-sdk-go/def/factory/defsvc"
	"github.com/hyperledger/fabric-sdk-go/pkg/errors"
	apisdk "github.com/hyperledger/fabric-sdk-go/pkg/fabsdk/api"
	"github.com/hyperledger/fabric-sdk-go/pkg/logging"

	chmgmt "github.com/hyperledger/fabric-sdk-go/api/apitxn/chmgmtclient"
	resmgmt "github.com/hyperledger/fabric-sdk-go/api/apitxn/resmgmtclient"
)

// Options encapsulates configuration for the SDK
type Options struct {
	// Quick access options
	ConfigFile string
	ConfigByte []byte
	ConfigType string

	// Options for default providers
	StateStoreOpts StateStoreOpts

	// Factories to create clients and providers
	CoreFactory    apisdk.CoreProviderFactory
	ServiceFactory apisdk.ServiceProviderFactory
	ContextFactory apisdk.OrgClientFactory
	SessionFactory apisdk.SessionClientFactory

	// Factories for creating package-level utilities (keep this to a minimum)
	// TODO: Should the logger actually be in ProviderFactory
	LoggerFactory apilogging.LoggerProvider
}

// StateStoreOpts provides setup parameters for KeyValueStore
type StateStoreOpts struct {
	Path string
}

// FabricSDK provides access (and context) to clients being managed by the SDK
type FabricSDK struct {
	Options

	// Implementations of client functionality (defaults are used if not specified)
	configProvider    apiconfig.Config
	stateStore        apifabclient.KeyValueStore
	cryptoSuite       apicryptosuite.CryptoSuite
	discoveryProvider apifabclient.DiscoveryProvider
	selectionProvider apifabclient.SelectionProvider
	signingManager    apifabclient.SigningManager
	fabricProvider    apicore.FabricProvider
}

// ChannelClientOpts provides options for creating channel client
type ChannelClientOpts struct {
	OrgName        string
	ConfigProvider apiconfig.Config
}

// ChannelMgmtClientOpts provides options for creating channel management client
type ChannelMgmtClientOpts struct {
	OrgName        string
	ConfigProvider apiconfig.Config
}

// ResourceMgmtClientOpts provides options for creating resource management client
type ResourceMgmtClientOpts struct {
	OrgName        string
	TargetFilter   resmgmt.TargetFilter
	ConfigProvider apiconfig.Config
}

// ProviderInit interface allows for initializing providers
type ProviderInit interface {
	Initialize(sdk *FabricSDK) error
}

// NewSDK initializes default clients
// TODO: Refactor option style
func NewSDK(options Options) (*FabricSDK, error) {
	// Construct SDK opts from the quick access options in setup
	sdkOpts := apisdk.SDKOpts{
		ConfigFile:  options.ConfigFile,
		ConfigBytes: options.ConfigByte,
		ConfigType:  options.ConfigType,
	}

	sdk := FabricSDK{
		Options: options,
	}

	// Initialize logging provider with default logging provider (if needed)
	if sdk.LoggerFactory == nil {
		sdk.LoggerFactory = defcore.NewLoggerProvider()
	}
	logging.InitLogger(sdk.LoggerFactory)

	// Initialize default factories (if needed)
	if sdk.CoreFactory == nil {
		sdk.CoreFactory = defcore.NewProviderFactory()
	}
	if sdk.ServiceFactory == nil {
		sdk.ServiceFactory = defsvc.NewProviderFactory()
	}
	if sdk.ContextFactory == nil {
		sdk.ContextFactory = defclient.NewOrgClientFactory()
	}
	if sdk.SessionFactory == nil {
		sdk.SessionFactory = defclient.NewSessionClientFactory()
	}

	// Initialize config provider
	config, err := sdk.CoreFactory.NewConfigProvider(sdkOpts)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize config")
	}
	sdk.configProvider = config

	// Initialize crypto provider
	cryptosuite, err := sdk.CoreFactory.NewCryptoSuiteProvider(sdk.configProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize crypto suite")
	}
	sdk.cryptoSuite = cryptosuite

	// Initialize state store
	stateStoreOpts := apisdk.StateStoreOpts{
		Path: sdk.StateStoreOpts.Path,
	}
	store, err := sdk.CoreFactory.NewStateStoreProvider(stateStoreOpts, sdk.configProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize state store")
	}
	sdk.stateStore = store

	// Initialize Signing Manager
	signingMgr, err := sdk.CoreFactory.NewSigningManager(sdk.CryptoSuiteProvider(), sdk.configProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize signing manager")
	}
	sdk.signingManager = signingMgr

	// Initialize Fabric Provider
	fabricProvider, err := sdk.CoreFactory.NewFabricProvider(sdk.configProvider, sdk.stateStore, sdk.cryptoSuite, sdk.signingManager)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize core fabric provider")
	}
	sdk.fabricProvider = fabricProvider

	// Initialize discovery provider
	discoveryProvider, err := sdk.ServiceFactory.NewDiscoveryProvider(sdk.configProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize discovery provider")
	}
	if pi, ok := discoveryProvider.(ProviderInit); ok {
		pi.Initialize(&sdk)
	}
	sdk.discoveryProvider = discoveryProvider

	// Initialize selection provider (for selecting endorsing peers)
	selectionProvider, err := sdk.ServiceFactory.NewSelectionProvider(sdk.configProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize selection provider")
	}
	if pi, ok := selectionProvider.(ProviderInit); ok {
		pi.Initialize(&sdk)
	}
	sdk.selectionProvider = selectionProvider

	return &sdk, nil
}

// ConfigProvider returns the Config provider of sdk.
func (sdk *FabricSDK) ConfigProvider() apiconfig.Config {
	return sdk.configProvider
}

// CryptoSuiteProvider returns the BCCSP provider of sdk.
func (sdk *FabricSDK) CryptoSuiteProvider() apicryptosuite.CryptoSuite {
	return sdk.cryptoSuite
}

// StateStoreProvider returns state store
func (sdk *FabricSDK) StateStoreProvider() apifabclient.KeyValueStore {
	return sdk.stateStore
}

// DiscoveryProvider returns discovery provider
func (sdk *FabricSDK) DiscoveryProvider() apifabclient.DiscoveryProvider {
	return sdk.discoveryProvider
}

// SelectionProvider returns selection provider
func (sdk *FabricSDK) SelectionProvider() apifabclient.SelectionProvider {
	return sdk.selectionProvider
}

// SigningManager returns signing manager
func (sdk *FabricSDK) SigningManager() apifabclient.SigningManager {
	return sdk.signingManager
}

// FabricProvider provides fabric objects such as peer and user
func (sdk *FabricSDK) FabricProvider() apicore.FabricProvider {
	return sdk.fabricProvider
}

// NewContext creates a context from an org
func (sdk *FabricSDK) NewContext(orgID string) (*OrgContext, error) {
	return newOrgContext(sdk.ContextFactory, orgID, sdk.configProvider)
}

// NewSession creates a session from a context and a user (TODO)
func (sdk *FabricSDK) NewSession(c apisdk.Org, user apifabclient.User) (*Session, error) {
	return newSession(user, sdk.SessionFactory), nil
}

// NewSystemClient returns a new client for the system (operations not on a channel)
// TODO: Reduced immutable interface
// TODO: Parameter for setting up the peers
func (sdk *FabricSDK) NewSystemClient(s apisdk.Session) (apifabclient.FabricClient, error) {
	return sdk.FabricProvider().NewClient(s.Identity())
}

// NewChannelMgmtClient returns a new client for managing channels
func (sdk *FabricSDK) NewChannelMgmtClient(userName string) (chmgmt.ChannelMgmtClient, error) {

	// Read default org name from configuration
	client, err := sdk.configProvider.Client()
	if err != nil {
		return nil, errors.WithMessage(err, "unable to retrieve client from network config")
	}

	if client.Organization == "" {
		return nil, errors.New("must provide default organisation name in configuration")
	}

	opt := &ChannelMgmtClientOpts{OrgName: client.Organization, ConfigProvider: sdk.configProvider}

	return sdk.NewChannelMgmtClientWithOpts(userName, opt)
}

// NewChannelMgmtClientWithOpts returns a new client for managing channels with options
func (sdk *FabricSDK) NewChannelMgmtClientWithOpts(userName string, opt *ChannelMgmtClientOpts) (chmgmt.ChannelMgmtClient, error) {

	if opt == nil || opt.OrgName == "" {
		return nil, errors.New("organization name must be provided")
	}

	session, err := sdk.NewPreEnrolledUserSession(opt.OrgName, userName)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pre-enrolled user session")
	}

	configProvider := sdk.ConfigProvider()
	if opt.ConfigProvider != nil {
		configProvider = opt.ConfigProvider
	}

	client, err := sdk.SessionFactory.NewChannelMgmtClient(sdk, session, configProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create new channel management client")
	}

	return client, nil
}

// NewResourceMgmtClient returns a new client for managing system resources
func (sdk *FabricSDK) NewResourceMgmtClient(userName string) (resmgmt.ResourceMgmtClient, error) {

	// Read default org name from configuration
	client, err := sdk.configProvider.Client()
	if err != nil {
		return nil, errors.WithMessage(err, "unable to retrieve client from network config")
	}

	if client.Organization == "" {
		return nil, errors.New("must provide default organisation name in configuration")
	}

	opt := &ResourceMgmtClientOpts{OrgName: client.Organization, ConfigProvider: sdk.configProvider}

	return sdk.NewResourceMgmtClientWithOpts(userName, opt)
}

// NewResourceMgmtClientWithOpts returns a new resource management client (user has to be pre-enrolled)
func (sdk *FabricSDK) NewResourceMgmtClientWithOpts(userName string, opt *ResourceMgmtClientOpts) (resmgmt.ResourceMgmtClient, error) {

	if opt == nil || opt.OrgName == "" {
		return nil, errors.New("organization name must be provided")
	}

	session, err := sdk.NewPreEnrolledUserSession(opt.OrgName, userName)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pre-enrolled user session")
	}

	configProvider := sdk.ConfigProvider()
	if opt.ConfigProvider != nil {
		configProvider = opt.ConfigProvider
	}

	client, err := sdk.SessionFactory.NewResourceMgmtClient(sdk, session, configProvider, opt.TargetFilter)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to created new resource management client")
	}

	return client, nil
}

// NewChannelClient returns a new client for a channel
func (sdk *FabricSDK) NewChannelClient(channelID string, userName string) (apitxn.ChannelClient, error) {

	// Read default org name from configuration
	client, err := sdk.configProvider.Client()
	if err != nil {
		return nil, errors.WithMessage(err, "unable to retrieve client from network config")
	}

	if client.Organization == "" {
		return nil, errors.New("must provide default organisation name in configuration")
	}

	opt := &ChannelClientOpts{OrgName: client.Organization, ConfigProvider: sdk.configProvider}

	return sdk.NewChannelClientWithOpts(channelID, userName, opt)
}

// NewChannelClientWithOpts returns a new client for a channel (user has to be pre-enrolled)
func (sdk *FabricSDK) NewChannelClientWithOpts(channelID string, userName string, opt *ChannelClientOpts) (apitxn.ChannelClient, error) {

	if opt == nil || opt.OrgName == "" {
		return nil, errors.New("organization name must be provided")
	}

	session, err := sdk.NewPreEnrolledUserSession(opt.OrgName, userName)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pre-enrolled user session")
	}

	configProvider := sdk.ConfigProvider()
	if opt.ConfigProvider != nil {
		configProvider = opt.ConfigProvider
	}

	client, err := sdk.SessionFactory.NewChannelClient(sdk, session, configProvider, channelID)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to created new channel client")
	}

	return client, nil
}

// NewPreEnrolledUser returns a new pre-enrolled user
// TODO: Rename this func to NewUser
func (sdk *FabricSDK) NewPreEnrolledUser(orgID string, userName string) (apifabclient.User, error) {

	credentialMgr, err := sdk.ContextFactory.NewCredentialManager(orgID, sdk.ConfigProvider(), sdk.CryptoSuiteProvider())
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get credential manager")
	}

	signingIdentity, err := credentialMgr.GetSigningIdentity(userName)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get signing identity")
	}

	user, err := sdk.FabricProvider().NewUser(userName, signingIdentity)
	if err != nil {
		return nil, errors.WithMessage(err, "NewPreEnrolledUser returned error")
	}

	return user, nil
}

// NewPreEnrolledUserSession returns a new pre-enrolled user session
// TODO: Rename this func to NewUserSession
func (sdk *FabricSDK) NewPreEnrolledUserSession(orgID string, userName string) (*Session, error) {

	context, err := sdk.NewContext(orgID)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get context for org")
	}

	user, err := sdk.NewPreEnrolledUser(orgID, userName)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pre-enrolled user")
	}

	session, err := sdk.NewSession(context, user)
	if err != nil {
		return nil, errors.WithMessage(err, "NewSession returned error")
	}

	return session, nil
}

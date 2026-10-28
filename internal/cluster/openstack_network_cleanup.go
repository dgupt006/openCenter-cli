// Copyright 2026 OpenCenter contributors
// Licensed under the Apache License, Version 2.0 (the "License");

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"

	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

// openStackNetworkCleanupService removes only cluster-owned Octavia resources
// before OpenTofu attempts to remove the cluster network and router interface.
// CCM-created load balancers otherwise leave Neutron ports attached and can
// cause the OpenTofu destroy to hang on router-interface deletion.
type openStackNetworkCleanupService interface {
	CleanupClusterLoadBalancers(ctx context.Context, clusterName, networkID string) error
}

type gophercloudOpenStackNetworkCleanupService struct {
	loadBalancerClient *gophercloud.ServiceClient
}

func newGophercloudOpenStackNetworkCleanupService(creds *credentials.OpenStackCredentials) (openStackNetworkCleanupService, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint:            creds.AuthURL,
		ApplicationCredentialID:     creds.ApplicationCredentialID,
		ApplicationCredentialSecret: creds.ApplicationCredentialSecret,
		AllowReauth:                 true,
	}
	if creds.ApplicationCredentialID == "" {
		authOpts.Username = creds.Username
		authOpts.Password = creds.Password
		authOpts.DomainName = creds.Domain
		if authOpts.DomainName == "" {
			authOpts.DomainName = creds.UserDomainName
		}
		authOpts.TenantName = creds.ProjectName
		if authOpts.TenantName == "" {
			authOpts.TenantName = creds.TenantName
		}
	}

	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		return nil, fmt.Errorf("authenticate with OpenStack: %w", err)
	}
	provider.HTTPClient = http.Client{Timeout: 30 * time.Second}

	client, err := openstack.NewLoadBalancerV2(provider, gophercloud.EndpointOpts{Region: creds.Region})
	if err != nil {
		return nil, fmt.Errorf("create load balancer client: %w", err)
	}
	return &gophercloudOpenStackNetworkCleanupService{loadBalancerClient: client}, nil
}

func (s *gophercloudOpenStackNetworkCleanupService) CleanupClusterLoadBalancers(ctx context.Context, clusterName, networkID string) error {
	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	page, err := loadbalancers.List(s.loadBalancerClient, loadbalancers.ListOpts{VipNetworkID: networkID}).AllPages()
	if err != nil {
		return fmt.Errorf("list Octavia load balancers: %w", err)
	}
	items, err := loadbalancers.ExtractLoadBalancers(page)
	if err != nil {
		return fmt.Errorf("extract Octavia load balancers: %w", err)
	}

	for _, lb := range items {
		// The cluster name is part of the CCM-generated LB name. Keep both
		// predicates: the exact network prevents touching another cluster in the
		// project, and the name prevents touching unrelated LBs on this network.
		if networkID != "" && lb.VipNetworkID != networkID {
			continue
		}
		if !strings.Contains(lb.Name, clusterName) {
			continue
		}

		if err := loadbalancers.Delete(s.loadBalancerClient, lb.ID, loadbalancers.DeleteOpts{Cascade: true}).ExtractErr(); err != nil {
			return fmt.Errorf("delete Octavia load balancer %s (%s): %w", lb.ID, lb.Name, err)
		}
		if err := waitForLoadBalancerDeletion(ctx, s.loadBalancerClient, lb.ID); err != nil {
			return fmt.Errorf("wait for Octavia load balancer %s deletion: %w", lb.ID, err)
		}
	}
	return nil
}

func waitForLoadBalancerDeletion(ctx context.Context, client *gophercloud.ServiceClient, id string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()

	for {
		result := loadbalancers.Get(client, id)
		if result.StatusCode == http.StatusNotFound {
			return nil
		}
		if result.Err != nil {
			return result.Err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out after 2m")
		case <-ticker.C:
		}
	}
}

// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package admission implements OpenShell's operator-owned external-resource
// admission policy. It does not authorize host-path attachment.
package admission

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	workspacePlaceholder  = "${workspace}"
	LabelManagedBy        = "openshell.ai/managed-by"
	LabelGatewayID        = "openshell.ai/gateway-id"
	LabelSandboxWorkspace = "openshell.ai/sandbox-workspace"
	ConfigUsedLabel       = "openshell.ai/caller-driver-config-used"
	IdentitiesLabel       = "openshell.ai/resource-admission-identities"
)

var reservedLabelKeys = map[string]struct{}{
	LabelManagedBy: {}, LabelGatewayID: {}, LabelSandboxWorkspace: {},
}

// ResourceAdmissionConfig is shared by all compute drivers. A supplied label
// map replaces the defaults.
type ResourceAdmissionConfig struct {
	Enabled        bool              `json:"enabled"`
	RequiredLabels map[string]string `json:"required_labels"`
}

// DefaultResourceAdmissionConfig returns the upstream defaults.
func DefaultResourceAdmissionConfig() ResourceAdmissionConfig {
	return ResourceAdmissionConfig{Enabled: true, RequiredLabels: map[string]string{
		"openshell.ai/sandbox-attachable":           "true",
		"openshell.ai/sandbox-attachable-workspace": workspacePlaceholder,
	}}
}

// DriverAdmissionConfig is the effective policy acknowledged by a driver.
type DriverAdmissionConfig struct {
	AllowDriverConfig bool                    `json:"allow_driver_config"`
	ResourceAdmission ResourceAdmissionConfig `json:"resource_admission"`
}

// DefaultDriverAdmissionConfig returns a secure default policy.
func DefaultDriverAdmissionConfig() DriverAdmissionConfig {
	return DriverAdmissionConfig{ResourceAdmission: DefaultResourceAdmissionConfig()}
}

// ParseDriverAdmissionConfig decodes a config while applying OpenShell defaults
// to omitted fields and rejecting unknown keys.
func ParseDriverAdmissionConfig(value []byte) (DriverAdmissionConfig, error) {
	config := DefaultDriverAdmissionConfig()
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return DriverAdmissionConfig{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return DriverAdmissionConfig{}, errors.New("invalid trailing JSON data")
	}
	if err := config.Validate(); err != nil {
		return DriverAdmissionConfig{}, err
	}
	return config, nil
}

// Validate checks the admission labels and rejects reserved driver metadata.
func (c ResourceAdmissionConfig) Validate() error {
	if c.Enabled && len(c.RequiredLabels) == 0 {
		return errors.New("resource_admission.required_labels must not be empty while enabled")
	}
	keys := make([]string, 0, len(c.RequiredLabels))
	for key := range c.RequiredLabels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	allWorkspace := len(keys) > 0
	for _, key := range keys {
		value := c.RequiredLabels[key]
		if _, reserved := reservedLabelKeys[key]; reserved {
			return fmt.Errorf("resource admission label key is reserved for driver-owned metadata: %s", key)
		}
		if !validLabelKey(key) {
			return fmt.Errorf("invalid resource admission label key: %s", key)
		}
		if value != workspacePlaceholder && !validLabelValue(value) {
			return fmt.Errorf("invalid resource admission label value for %s; only whole-value ${workspace} substitution is supported", key)
		}
		if value != workspacePlaceholder {
			allWorkspace = false
		}
	}
	if c.Enabled && allWorkspace {
		return errors.New("resource_admission.required_labels must include a shared approval label")
	}
	return nil
}

// Acknowledgement returns the versioned canonical JSON policy.
func (c DriverAdmissionConfig) Acknowledgement() string {
	if c.ResourceAdmission.RequiredLabels == nil {
		c.ResourceAdmission.RequiredLabels = map[string]string{}
	}
	data, err := json.Marshal(c)
	if err != nil {
		panic("admission policy contains only JSON-safe values")
	}
	return "v1:" + string(data)
}

// Validate checks the full policy before it is acknowledged or applied.
func (c DriverAdmissionConfig) Validate() error { return c.ResourceAdmission.Validate() }

// VerifyAcknowledgement requires the driver's parsed policy to match exactly.
func (c DriverAdmissionConfig) VerifyAcknowledgement(value string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if !c.ResourceAdmission.Enabled && value == "" {
		return nil
	}
	if !strings.HasPrefix(value, "v1:") {
		return errors.New("compute driver does not acknowledge resource admission v1")
	}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(value, "v1:")))
	decoder.DisallowUnknownFields()
	got := DefaultDriverAdmissionConfig()
	if err := decoder.Decode(&got); err != nil {
		return errors.New("invalid driver admission acknowledgement")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("invalid driver admission acknowledgement")
	}
	if err := got.Validate(); err != nil {
		return errors.New("invalid driver admission acknowledgement")
	}
	if !equalConfig(got, c) {
		return errors.New("compute driver admission policy differs from gateway configuration")
	}
	return nil
}

// Admit evaluates labels from the trusted resource authority for a workspace.
func (c ResourceAdmissionConfig) Admit(workspace string, labels map[string]string) error {
	return c.admit(&workspace, labels)
}

// AdmitShared evaluates a cluster-scoped shared resource. Workspace-scoped
// requirements are skipped, while fixed approval labels still apply.
func (c ResourceAdmissionConfig) AdmitShared(labels map[string]string) error {
	return c.admit(nil, labels)
}

func (c ResourceAdmissionConfig) admit(workspace *string, labels map[string]string) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("failed precondition: %w", err)
	}
	if !c.Enabled {
		return nil
	}
	keys := make([]string, 0, len(c.RequiredLabels))
	for key := range c.RequiredLabels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		expected := c.RequiredLabels[key]
		if expected == workspacePlaceholder {
			if workspace == nil {
				continue
			}
			if *workspace == "" || !validLabelValue(*workspace) {
				return errors.New("failed precondition: resource not admitted: invalid workspace identity")
			}
			expected = *workspace
		}
		if labels[key] != expected {
			return errors.New("failed precondition: external resource not admitted by required labels")
		}
	}
	return nil
}

// RejectUnlabelable prevents attaching resources that lack a trusted label resolver.
func (c ResourceAdmissionConfig) RejectUnlabelable(kind string) error {
	if c.Enabled {
		return fmt.Errorf("failed precondition: %s cannot be attached while resource admission is enabled: no trusted label resolver", kind)
	}
	return nil
}

// CheckConfigProvenance rejects old sandboxes with forbidden or missing provenance.
func CheckConfigProvenance(allowed bool, recorded *string) error {
	if recorded != nil && (*recorded == "false" || (*recorded == "true" && allowed)) {
		return nil
	}
	return errors.New("failed precondition: sandbox uses forbidden driver config or lacks admission provenance; recreate it")
}

// CheckDriverConfig gates caller-authored backend-specific JSON independently of
// label-based admission.
func CheckDriverConfig(allowed bool, fields map[string]any) error {
	if !allowed && len(fields) > 0 {
		return errors.New("failed precondition: caller driver config is disabled; a gateway administrator must enable allow_driver_config")
	}
	return nil
}

func equalConfig(a, b DriverAdmissionConfig) bool {
	if a.AllowDriverConfig != b.AllowDriverConfig || a.ResourceAdmission.Enabled != b.ResourceAdmission.Enabled || len(a.ResourceAdmission.RequiredLabels) != len(b.ResourceAdmission.RequiredLabels) {
		return false
	}
	for key, value := range a.ResourceAdmission.RequiredLabels {
		if b.ResourceAdmission.RequiredLabels[key] != value {
			return false
		}
	}
	return true
}

func validLabelValue(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 63 || !asciiAlphaNumeric(value[0]) || !asciiAlphaNumeric(value[len(value)-1]) {
		return false
	}
	for i := range value {
		b := value[i]
		if !asciiAlphaNumeric(b) && b != '-' && b != '_' && b != '.' {
			return false
		}
	}
	return true
}

func validLabelKey(key string) bool {
	prefix, name, hasPrefix := strings.Cut(key, "/")
	if !validLabelValue(name) || name == "" {
		return false
	}
	if !hasPrefix {
		return true
	}
	if prefix == "" || len(prefix) > 253 {
		return false
	}
	for _, part := range strings.Split(prefix, ".") {
		if part == "" || len(part) > 63 || !asciiAlphaNumeric(part[0]) || !asciiAlphaNumeric(part[len(part)-1]) {
			return false
		}
		for i := range part {
			b := part[i]
			if (b < 'a' || b > 'z') && (b < '0' || b > '9') && b != '-' {
				return false
			}
		}
	}
	return true
}

func asciiAlphaNumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

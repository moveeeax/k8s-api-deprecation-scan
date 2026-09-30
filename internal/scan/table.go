package scan

import "strings"

// Rule describes one GroupVersionKind that upstream Kubernetes is retiring.
//
// DeprecatedIn is the minor release where the API was marked deprecated.
// RemovedIn is the minor release where the API stopped being served; an empty
// RemovedIn means a removal release has not been announced yet.
type Rule struct {
	APIVersion   string `json:"apiVersion"`
	Kind         string `json:"kind"`
	DeprecatedIn string `json:"deprecatedIn"`
	RemovedIn    string `json:"removedIn,omitempty"`
	Replacement  string `json:"replacement,omitempty"`
}

func kinds(apiVersion, deprecatedIn, removedIn, replacement string, ks ...string) []Rule {
	out := make([]Rule, 0, len(ks))
	for _, k := range ks {
		out = append(out, Rule{
			APIVersion:   apiVersion,
			Kind:         k,
			DeprecatedIn: deprecatedIn,
			RemovedIn:    removedIn,
			Replacement:  replacement,
		})
	}
	return out
}

// builtinRules is the table the scanner checks against. Sources: the upstream
// "Deprecated API Migration Guide" for each minor release.
var builtinRules = concat(
	// Removed in 1.16.
	kinds("apps/v1beta1", "1.9", "1.16", "apps/v1", "Deployment", "DaemonSet", "ReplicaSet", "StatefulSet"),
	kinds("apps/v1beta2", "1.9", "1.16", "apps/v1", "Deployment", "DaemonSet", "ReplicaSet", "StatefulSet"),
	kinds("extensions/v1beta1", "1.9", "1.16", "apps/v1", "Deployment", "DaemonSet", "ReplicaSet"),
	kinds("extensions/v1beta1", "1.9", "1.16", "networking.k8s.io/v1", "NetworkPolicy"),
	kinds("extensions/v1beta1", "1.9", "1.16", "policy/v1beta1", "PodSecurityPolicy"),

	// Removed in 1.22.
	kinds("admissionregistration.k8s.io/v1beta1", "1.16", "1.22", "admissionregistration.k8s.io/v1", "MutatingWebhookConfiguration", "ValidatingWebhookConfiguration"),
	kinds("apiextensions.k8s.io/v1beta1", "1.16", "1.22", "apiextensions.k8s.io/v1", "CustomResourceDefinition"),
	kinds("apiregistration.k8s.io/v1beta1", "1.19", "1.22", "apiregistration.k8s.io/v1", "APIService"),
	kinds("authentication.k8s.io/v1beta1", "1.19", "1.22", "authentication.k8s.io/v1", "TokenReview"),
	kinds("authorization.k8s.io/v1beta1", "1.19", "1.22", "authorization.k8s.io/v1",
		"LocalSubjectAccessReview", "SelfSubjectAccessReview", "SelfSubjectRulesReview", "SubjectAccessReview"),
	kinds("certificates.k8s.io/v1beta1", "1.19", "1.22", "certificates.k8s.io/v1", "CertificateSigningRequest"),
	kinds("coordination.k8s.io/v1beta1", "1.19", "1.22", "coordination.k8s.io/v1", "Lease"),
	kinds("extensions/v1beta1", "1.14", "1.22", "networking.k8s.io/v1", "Ingress"),
	kinds("networking.k8s.io/v1beta1", "1.19", "1.22", "networking.k8s.io/v1", "Ingress", "IngressClass"),
	kinds("rbac.authorization.k8s.io/v1beta1", "1.17", "1.22", "rbac.authorization.k8s.io/v1",
		"ClusterRole", "ClusterRoleBinding", "Role", "RoleBinding"),
	kinds("scheduling.k8s.io/v1beta1", "1.17", "1.22", "scheduling.k8s.io/v1", "PriorityClass"),
	kinds("storage.k8s.io/v1beta1", "1.19", "1.22", "storage.k8s.io/v1", "CSIDriver", "CSINode", "StorageClass", "VolumeAttachment"),

	// Removed in 1.25.
	kinds("batch/v1beta1", "1.21", "1.25", "batch/v1", "CronJob"),
	kinds("discovery.k8s.io/v1beta1", "1.21", "1.25", "discovery.k8s.io/v1", "EndpointSlice"),
	kinds("events.k8s.io/v1beta1", "1.19", "1.25", "events.k8s.io/v1", "Event"),
	kinds("autoscaling/v2beta1", "1.22", "1.25", "autoscaling/v2", "HorizontalPodAutoscaler"),
	kinds("policy/v1beta1", "1.21", "1.25", "policy/v1", "PodDisruptionBudget"),
	kinds("policy/v1beta1", "1.21", "1.25", "", "PodSecurityPolicy"),
	kinds("node.k8s.io/v1beta1", "1.22", "1.25", "node.k8s.io/v1", "RuntimeClass"),

	// Removed in 1.26.
	kinds("autoscaling/v2beta2", "1.23", "1.26", "autoscaling/v2", "HorizontalPodAutoscaler"),
	kinds("flowcontrol.apiserver.k8s.io/v1beta1", "1.23", "1.26", "flowcontrol.apiserver.k8s.io/v1", "FlowSchema", "PriorityLevelConfiguration"),

	// Removed in 1.27.
	kinds("storage.k8s.io/v1beta1", "1.24", "1.27", "storage.k8s.io/v1", "CSIStorageCapacity"),

	// Removed in 1.29.
	kinds("flowcontrol.apiserver.k8s.io/v1beta2", "1.26", "1.29", "flowcontrol.apiserver.k8s.io/v1", "FlowSchema", "PriorityLevelConfiguration"),

	// Removed in 1.32.
	kinds("flowcontrol.apiserver.k8s.io/v1beta3", "1.29", "1.32", "flowcontrol.apiserver.k8s.io/v1", "FlowSchema", "PriorityLevelConfiguration"),
)

func concat(groups ...[]Rule) []Rule {
	var out []Rule
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// Table indexes rules by apiVersion + kind.
type Table struct {
	byKey map[string]Rule
	rules []Rule
}

// BuiltinTable returns the compiled-in deprecation table.
func BuiltinTable() *Table {
	t := &Table{byKey: make(map[string]Rule, len(builtinRules)), rules: builtinRules}
	for _, r := range builtinRules {
		t.byKey[ruleKey(r.APIVersion, r.Kind)] = r
	}
	return t
}

// Rules returns every rule in the table, in declaration order.
func (t *Table) Rules() []Rule { return t.rules }

// Lookup finds the rule for an apiVersion/kind pair, if the API is retiring.
func (t *Table) Lookup(apiVersion, kind string) (Rule, bool) {
	r, ok := t.byKey[ruleKey(apiVersion, kind)]
	return r, ok
}

func ruleKey(apiVersion, kind string) string {
	return strings.TrimSpace(apiVersion) + "|" + strings.TrimSpace(kind)
}

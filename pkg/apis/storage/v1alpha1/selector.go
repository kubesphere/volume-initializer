package v1alpha1

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

const (
	FieldName      = "name"
	FieldNamespace = "namespace"
)

// GenericSelector supports field selector and label selector, they're ANDed requirements.
type GenericSelector struct {
	// FieldSelector is the field selector, which only supports "name" and "namespace" as key, and "In" and "NotIn" as operator.
	FieldSelector []metav1.FieldSelectorRequirement `json:"fieldSelector,omitempty"`

	// LabelSelector is the label selector
	LabelSelector []metav1.LabelSelectorRequirement `json:"labelSelector,omitempty"`
}

func (s *GenericSelector) Match(obj metav1.Object) bool {
	if obj == nil {
		return false
	}

	for _, req := range s.FieldSelector {
		if req.Key != FieldName && req.Key != FieldNamespace {
			continue
		}
		if req.Operator != metav1.FieldSelectorOpIn && req.Operator != metav1.FieldSelectorOpNotIn {
			continue
		}

		var val string
		if req.Key == FieldName {
			val = obj.GetName()
		}
		if req.Key == FieldNamespace {
			val = obj.GetNamespace()
		}

		var match bool
		if req.Operator == metav1.FieldSelectorOpIn {
			match = slices.Contains(req.Values, val)
		}
		if req.Operator == metav1.FieldSelectorOpNotIn {
			match = !slices.Contains(req.Values, val)
		}

		if !match {
			return false
		}
	}

	if len(s.LabelSelector) > 0 {
		labelSelector := metav1.LabelSelector{
			MatchExpressions: s.LabelSelector,
		}
		selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
		if err != nil {
			klog.ErrorS(err, "LabelSelectorAsSelector", "labelSelector", labelSelector)
			return false
		}
		match := selector.Matches(labels.Set(obj.GetLabels()))
		if !match {
			return false
		}
	}

	return true
}

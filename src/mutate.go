package src

import (
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *TLSServer) Mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {

	return &admissionv1.AdmissionResponse{
		Allowed: allowed,
		Result: &metav1.Status{
			Code:    int32(code),
			Message: message,
		},
	}
}

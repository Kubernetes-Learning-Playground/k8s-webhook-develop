package pkg

import (
	"encoding/json"
	"fmt"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"net/http"
	"os"
	"strings"
)

const (
	AnnotationMutateKey = "my.webhook.practice.admission-registry/mutate" // my.webhook.admission-registry/mutate = no/off/false/n
	AnnotationStatusKey = "my.webhook.practice.admission-registry/status" // my.webhook.practice.admission-registry/status = mutated
	LabelMutateKey      = "my.webhook.practice.admission-registry/mutate" // my.webhook.admission-registry/mutate = no/off/false/n
	LabelStatusKey      = "my.webhook.practice.admission-registry/status" // my.webhook.practice.admission-registry/status = mutated
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

var (
	objectMeta     *metav1.ObjectMeta
	needSidecarPod *corev1.Pod
)

func (s *TLSServer) Mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {

	req := ar.Request

	klog.Infof("AdmissionReview for Kind=%s, Namespace=%s Name=%v UID=%v Operation=%v UserInfo=%v",
		req.Kind.Kind, req.Namespace, req.Name, req.UID, req.Operation, req.UserInfo)

	// 区分传入资源类型
	switch req.Kind.Kind {
	case "Deployment":
		var deployment appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			klog.Errorf("Can't not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			}

		}

		objectMeta = &deployment.ObjectMeta

	case "Service":
		var service corev1.Service
		if err := json.Unmarshal(req.Object.Raw, &service); err != nil {
			klog.Errorf("Can't not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			}
		}

		objectMeta = &service.ObjectMeta
	case "Pod":
		var pod corev1.Pod
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			klog.Errorf("Can't not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			}
		}

		objectMeta = &pod.ObjectMeta
		needSidecarPod = &pod

	default:
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Can't handle the kind(%s) object", req.Kind.Kind),
			},
		}
	}

	// 判断mutate功能

	if s.MutateObject == "annotation" {
		need := mutationAnnotationRequired(objectMeta)
		if !need {
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
		res := useMutateAnnotation(objectMeta)

		return res

	} else if s.MutateObject == "label" {
		need := mutationLabelRequired(objectMeta)
		if !need {
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
		res := useMutateLabel(objectMeta)

		return res
	} else if s.MutateObject == "image" && req.Kind.Kind == "Pod" {

		if os.Getenv("MUTATE_PATCH_IMAGE_REPLACE") == "false" {
			// 如果是sidecar模式 直接用这个
			return Sidecar(needSidecarPod)

		}

		res := patchContainerImage()

		klog.Infof("patch res", string(res))

		return &admissionv1.AdmissionResponse{
			Allowed: true,
			Patch:   res,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		}

	} else if s.MutateObject == "image" && req.Kind.Kind != "Pod" {
		return &admissionv1.AdmissionResponse{
			Allowed: allowed,
			Result: &metav1.Status{
				Code:    int32(code),
				Message: fmt.Sprintf("Can't handle the kind(%s) object in change image", req.Kind.Kind),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: allowed,
		Result: &metav1.Status{
			Code:    int32(code),
			Message: message,
		},
	}
}

func mutationAnnotationRequired(metadata *metav1.ObjectMeta) bool {

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	var need bool

	switch strings.ToLower(annotations[AnnotationMutateKey]) {
	case "n", "no", "false", "off":
		need = false
	default:
		need = true

	}

	status := annotations[AnnotationStatusKey]
	if strings.ToLower(status) == "mutated" {
		need = false
	}

	klog.Infof("Mutation policy for %s/%s: required: %v", metadata.Name, metadata.Namespace, need)

	return need

}

func mutationLabelRequired(metadata *metav1.ObjectMeta) bool {

	labels := metadata.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	var need bool

	switch strings.ToLower(labels[LabelMutateKey]) {
	case "n", "no", "false", "off":
		need = false
	default:
		need = true

	}

	status := labels[LabelStatusKey]
	if strings.ToLower(status) == "mutated" {
		need = false
	}

	klog.Infof("Mutation policy for %s/%s: required: %v", metadata.Name, metadata.Namespace, need)

	return need

}

func parseCustomize(customizeAnnotation string) (string, string) {
	res := strings.Split(customizeAnnotation, ":")
	if len(res) < 2 {
		return "", ""
	}

	return res[0], res[1]
}

func useMutateAnnotation(objectMeta *metav1.ObjectMeta) *admissionv1.AdmissionResponse {

	customizeAnnotation := os.Getenv("ANNOTATION_KEY_VALUE")

	annotationKey, annotationValue := parseCustomize(customizeAnnotation)

	newAnnotations := map[string]string{
		AnnotationStatusKey: "mutated",
		annotationKey:       annotationValue,
	}

	var patch []patchOperation
	patch = append(patch, mutateAnnotations(objectMeta.GetAnnotations(), newAnnotations)...)
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		klog.Errorf("patch marshal error: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			},
		}
	}
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}

}

func useMutateLabel(objectMeta *metav1.ObjectMeta) *admissionv1.AdmissionResponse {

	customizeLabel := os.Getenv("LABEL_KEY_VALUE")

	labelKey, labelValue := parseCustomize(customizeLabel)

	newLabels := map[string]string{
		LabelStatusKey: "mutated",
		labelKey:       labelValue,
	}

	var patch []patchOperation
	patch = append(patch, mutateLabels(objectMeta.GetLabels(), newLabels)...)
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		klog.Errorf("patch marshal error: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			},
		}
	}
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}

}

func mutateAnnotations(target map[string]string, added map[string]string) (patch []patchOperation) {

	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return
}

func mutateLabels(target map[string]string, added map[string]string) (patch []patchOperation) {

	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/labels",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/labels/" + key,
				Value: value,
			})
		}
	}
	return
}

type PatchOperate struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type InjectionOperate struct {
	Op    string   `json:"op"`
	Path  string   `json:"path"`
	Value []*Value `json:"value"`
}

type Value struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command"`
}

func patchContainerImage() []byte {

	klog.Info("patch the container image.....")

	patch := &PatchOperate{}
	//sidecarPatch := &InjectionOperate{}
	var b1 []byte
	// 区分是替换image模式还是sidecar模式
	if os.Getenv("MUTATE_PATCH_IMAGE_REPLACE") == "true" {
		klog.Info("use replace container image.....")
		patch = &PatchOperate{
			Op:    "replace",
			Path:  "/spec/containers/0/image",
			Value: os.Getenv("MUTATE_PATCH_IMAGE"), // 从环境变量取image
		}

		b1, _ = json.Marshal(patch)
		klog.Info("patch marshal: ", string(b1))

	}

	return b1
}

func parseInitContainerCommand(commandString string) []string {
	res := make([]string, 0)
	splitString := strings.Split(commandString, ",")
	for _, v := range splitString {
		res = append(res, v)
	}
	return res
}

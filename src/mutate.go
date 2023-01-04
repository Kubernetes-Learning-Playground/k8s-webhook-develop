package src

import (
	"context"
	"encoding/json"
	"fmt"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8sWebhookPractice/client"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	AnnotationMutateKey = "my.webhook.practice.admission-registry/mutate" // my.webhook.admission-registry/mutate = no/off/false/n
	AnnotationStatusKey = "my.webhook.practice.admission-registry/status" // my.webhook.practice.admission-registry/status = mutated
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

var (
	objectMeta *metav1.ObjectMeta
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

	default:
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Can't handle the kind(%s) object", req.Kind.Kind),
			},
		}
	}

	// 判断mutate功能

	if s.AnnotationOrImage == "annotation" {
		need := mutationRequired(objectMeta)
		if !need {
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
		res := useMutateAnnotation(objectMeta)

		return res

	} else if s.AnnotationOrImage == "image" && req.Kind.Kind == "Pod" {

		podName := req.Name
		podNamespace := req.Namespace

		// 取pod

		pod, err := client.ClientSet.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})

		var containerLen int

		// 如果错误是没找到
		if err != nil && errors.IsNotFound(err) {
			//return &admissionv1.AdmissionResponse{
			//	Allowed: allowed,
			//	Result: &metav1.Status{
			//		Code:    int32(code),
			//		Message: message,
			//	},
			//}
		} else if err!= nil && !errors.IsNotFound(err) {
			// 别的错误直接返回
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code: http.StatusBadRequest,
					Message: err.Error(),
				},
			}
		} else {
			// 如果有找到，取容器数量
			containerLen = len(pod.Spec.Containers)
		}

		res := patchContainerImage(containerLen)

		klog.Infof("patch res", string(res))

		return &admissionv1.AdmissionResponse{
			Allowed: true,
			Patch: res,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		}

	} else if s.AnnotationOrImage == "image" && req.Kind.Kind != "Pod" {
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

func mutationRequired(metadata *metav1.ObjectMeta) bool {

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

func parseCustomizeAnnotation(customizeAnnotation string) (string, string) {
	res := strings.Split(customizeAnnotation, ":")
	if len(res) < 2 {
		return "", ""
	}

	return res[0], res[1]
}

func useMutateAnnotation(objectMeta *metav1.ObjectMeta) *admissionv1.AdmissionResponse {

	customizeAnnotation := os.Getenv("ANNOTATION_KEY_VALUE")

	annotationKey, annotationValue := parseCustomizeAnnotation(customizeAnnotation)

	newAnnotations := map[string]string{
		AnnotationStatusKey: "mutated",
		annotationKey: annotationValue,
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

func patchContainerImage(containerLen int) []byte {

	klog.Info("patch the container image.....")

	patch := &PatchOperate{}

	// 区分是替换image模式还是sidecar模式
	if os.Getenv("MUTATE_PATCH_IMAGE_REPLACE") == "true" {
		klog.Info("use replace container image.....")
		patch = &PatchOperate{
			Op:    "replace",
			Path:  "/spec/containers/0/image",
			Value: os.Getenv("MUTATE_PATCH_IMAGE"), // 从环境变量取image
		}
	} else {
		klog.Info("use sidecar container image.....")
		// FIXME： sidecar模式还会报错
		num := containerLen + 1
		patch = &PatchOperate{
			Op:    "add",
			Path:  "/spec/containers" + "/" + strconv.Itoa(num) + "/image",
			Value: os.Getenv("MUTATE_PATCH_IMAGE"), // 从环境变量取image
		}
	}

	b1, err := json.Marshal(patch)
	if err != nil {
		log.Println(err)
		return []byte{}
	}
	var res []byte
	// FIXME 试验用的 init容器
	if os.Getenv("IS_INIT_IMAGE") == "true" {
		klog.Info("use init container image.....")
		valueList := make([]*Value, 0)
		value := &Value{
			Name:    "myinit",
			Image:   "busybox:1.28",
			Command: []string{"sh", "-c", "echo The app is running!"},
		}
		valueList = append(valueList, value)

		injection := &InjectionOperate{
			Op:    "add",
			Path:  "/spec/initContainers",
			Value: valueList,
		}
		b2, err := json.Marshal(injection)
		if err != nil {
			log.Println(err)
			return []byte{}
		}

		resString := "[" + string(b1) + "," + string(b2) + "]"
		res = []byte(resString)
		fmt.Println("init container + container", resString)
	} else {
		res = b1
		fmt.Println("container", string(res))
	}



	return res
}

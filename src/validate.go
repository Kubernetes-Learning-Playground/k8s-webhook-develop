package src

import (
	"encoding/json"
	"fmt"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"strings"
)

var (
	allowed = true
	code    = 200
	message = ""
)

// TODO: 目前仅支持替换镜像patch功能，未来预计新增类似istio的注入特定容器功能。

func (s *TLSServer) Validate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {

	req := ar.Request

	klog.Infof("AdmissionReview for Kind=%s, Namespace=%s Name=%v UID=%v Operation=%v UserInfo=%v",
		req.Kind.Kind, req.Namespace, req.Name, req.UID, req.Operation, req.UserInfo)

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		allowed = false
		code = 400
		return &admissionv1.AdmissionResponse{
			Allowed: allowed,
			Result: &metav1.Status{
				Code:    int32(code),
				Message: err.Error(),
			},
		}
	}
	var res *admissionv1.AdmissionResponse
	if s.WhiteOrBlock == "white" {
		res = s.useWhiteList(&pod)
	} else if s.WhiteOrBlock == "block" {
		res = s.useBlockList(&pod)
	}

	return res

}

// 遍例容器镜像列表，当镜像地址匹配到前缀，代表都是符合白名单的镜像列表

func (s *TLSServer) useWhiteList(pod *corev1.Pod) *admissionv1.AdmissionResponse {
	for _, container := range pod.Spec.Containers {
		var whiteListed = false
		for _, reg := range s.WhiteListRegistries {
			if strings.HasPrefix(container.Image, reg) {
				whiteListed = true
			}
		}
		if !whiteListed { // 如果不在白名单上，返回校验失败
			allowed = false
			code = 403
			message = fmt.Sprintf("whiteList, %s image comes from an untrusted registry! Only images from %v are allowed.",
				container.Image, s.WhiteListRegistries)
			break
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
// 遍例容器镜像列表，当镜像地址匹配到前缀，代表都是符合黑名单的镜像列表

func (s *TLSServer) useBlockList(pod *corev1.Pod) *admissionv1.AdmissionResponse {
	for _, container := range pod.Spec.Containers {
		var blockListed = false
		for _, reg := range s.BlackListRegistries {
			if strings.HasPrefix(container.Image, reg) {
				blockListed = true
			}
		}
		if blockListed { // 如果在黑名单上，返回校验失败
			allowed = false
			code = 403
			message = fmt.Sprintf("blackList, %s image comes from an untrusted registry! Only images from %v are allowed.",
				container.Image, s.WhiteListRegistries)
			break
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

// Code generated by wazero-emscripten-embind, DO NOT EDIT.
package gen

import "github.com/jerbob92/wazero-emscripten-embind"

func Attach(e embind.Engine) error {
	if err := e.RegisterClass("AlignedMemory", &ClassAlignedMemory{}); err != nil {
		return err
	}
	if err := e.RegisterClass("AlignedMemoryList", &ClassAlignedMemoryList{}); err != nil {
		return err
	}
	if err := e.RegisterClass("BlockingService", &ClassBlockingService{}); err != nil {
		return err
	}
	if err := e.RegisterClass("Response", &ClassResponse{}); err != nil {
		return err
	}
	if err := e.RegisterClass("TranslationModel", &ClassTranslationModel{}); err != nil {
		return err
	}
	if err := e.RegisterClass("VectorResponse", &ClassVectorResponse{}); err != nil {
		return err
	}
	if err := e.RegisterClass("VectorResponseOptions", &ClassVectorResponseOptions{}); err != nil {
		return err
	}
	if err := e.RegisterClass("VectorString", &ClassVectorString{}); err != nil {
		return err
	}
	return nil
}

package rag

// DefaultVectorDimensions 只在调用方没给出维度时兜底；冒烟脚本用 bge-m3 的真实维度（1024）覆盖它。
const DefaultVectorDimensions = 1024

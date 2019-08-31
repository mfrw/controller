#!/bin/bash
$HOME/g/src/k8s.io/code-generator/generate-groups.sh "all" \
	github.com/mfrw/controller/client-go/pkg/generated \
	github.com/mfrw/controller/client-go/pkg/apis \
	cnat:v1alpha1

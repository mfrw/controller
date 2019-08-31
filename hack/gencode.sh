#!/bin/bash
$HOME/g/src/k8s.io/code-generator/generate-groups.sh "deepcopy,client,informer,lister" github.com/mfrw/controller/pkg/generated github.com/mfrw/controller/pkg/apis cnat:v1alpha1

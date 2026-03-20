package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: compose-ephemeral <compose.yaml>")
		os.Exit(1)
	}

	input := os.Args[1]
	output := makeOverrideName(input)

	data, err := os.ReadFile(input)
	if err != nil {
		panic(err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		panic(err)
	}

	doc := root.Content[0]

	services := getMapValue(doc, "services")
	if services == nil {
		write(output, &root)
		return
	}

	// 新しい services を作る（portsだけ）
	newServices := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	for i := 0; i < len(services.Content); i += 2 {
		svcName := services.Content[i]
		svc := services.Content[i+1]

		portsNode := getMapValue(svc, "ports")
		if portsNode == nil || portsNode.Kind != yaml.SequenceNode {
			continue
		}

		newSeq := &yaml.Node{
			Kind: yaml.SequenceNode,
			Tag:  "!override",
		}

		for _, p := range portsNode.Content {
			switch p.Kind {
			case yaml.ScalarNode:
				parts := strings.Split(p.Value, ":")
				newSeq.Content = append(newSeq.Content, &yaml.Node{
					Kind:  yaml.ScalarNode,
					Tag:   "!!str",
					Value: ":" + parts[len(parts)-1],
				})

			case yaml.MappingNode:
				target := getMapValue(p, "target")
				if target != nil {
					newMap := &yaml.Node{
						Kind: yaml.MappingNode,
					}
					newMap.Content = append(newMap.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Value: "target"},
						target,
					)
					newSeq.Content = append(newSeq.Content, newMap)
				}
			}
		}

		// serviceごとに portsだけ持つ構造を作る
		newSvc := &yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "ports"},
				newSeq,
			},
		}

		newServices.Content = append(newServices.Content, svcName, newSvc)
	}

	// rootを最小構成にする
	newDoc := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "services"},
			newServices,
		},
	}

	root.Content[0] = newDoc

	write(output, &root)
	fmt.Println("Generated", output)
}

func makeOverrideName(input string) string {
	ext := filepath.Ext(input)
	base := strings.TrimSuffix(input, ext)
	return base + ".override" + ext
}

func getMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func write(path string, root *yaml.Node) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		panic(err)
	}
}

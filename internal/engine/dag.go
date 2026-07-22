package engine

import (
	"fmt"
	"goflow/internal/nodes"
)

type DAGPlan struct {
	Nodes        map[string]*nodes.Node
	Dependencies map[string][]string // NodeID -> danh sách NodeIDs phụ thuộc (các node phải chạy trước)
	Dependents   map[string][]string // NodeID -> danh sách NodeIDs phụ thuộc vào node này (các node chạy sau)
	InDegree     map[string]int      // Số lượng phụ thuộc đầu vào chưa hoàn thành
	ExecutionLayers [][]string        // Các lớp node có thể thực thi song song
}

// BuildDAGPlan phân tích danh sách Nodes và Edges, kiểm tra chu trình và tạo execution plan
func BuildDAGPlan(nodeList []nodes.Node, edgeList []nodes.Edge) (*DAGPlan, error) {
	nodeMap := make(map[string]*nodes.Node)
	inDegree := make(map[string]int)
	dependencies := make(map[string][]string)
	dependents := make(map[string][]string)

	for i := range nodeList {
		n := &nodeList[i]
		nodeMap[n.ID] = n
		inDegree[n.ID] = 0
		dependencies[n.ID] = []string{}
		dependents[n.ID] = []string{}
	}

	for _, edge := range edgeList {
		if _, srcExists := nodeMap[edge.Source]; !srcExists {
			return nil, fmt.Errorf("edge references non-existent source node: %s", edge.Source)
		}
		if _, tgtExists := nodeMap[edge.Target]; !tgtExists {
			return nil, fmt.Errorf("edge references non-existent target node: %s", edge.Target)
		}

		inDegree[edge.Target]++
		dependencies[edge.Target] = append(dependencies[edge.Target], edge.Source)
		dependents[edge.Source] = append(dependents[edge.Source], edge.Target)
	}

	// Kahn's Algorithm cho Topological Sorting và Cycle Detection
	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var executionLayers [][]string
	processedCount := 0

	inDegreeCopy := make(map[string]int)
	for k, v := range inDegree {
		inDegreeCopy[k] = v
	}

	for len(queue) > 0 {
		layerSize := len(queue)
		currentLayer := make([]string, 0, layerSize)

		for i := 0; i < layerSize; i++ {
			nodeID := queue[0]
			queue = queue[1:]

			currentLayer = append(currentLayer, nodeID)
			processedCount++

			for _, nextID := range dependents[nodeID] {
				inDegreeCopy[nextID]--
				if inDegreeCopy[nextID] == 0 {
					queue = append(queue, nextID)
				}
			}
		}

		executionLayers = append(executionLayers, currentLayer)
	}

	if processedCount != len(nodeList) {
		return nil, fmt.Errorf("detected cyclic dependency in workflow (DAG violation)")
	}

	return &DAGPlan{
		Nodes:           nodeMap,
		Dependencies:    dependencies,
		Dependents:      dependents,
		InDegree:        inDegree,
		ExecutionLayers: executionLayers,
	}, nil
}

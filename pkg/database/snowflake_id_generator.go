package database

import "github.com/bwmarrin/snowflake"

type snowFlake struct {
	node *snowflake.Node
}

func NewSnowFlakeIDGenerator(nodeID int64) (*snowFlake, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, err
	}
	return &snowFlake{node: node}, nil
}

func (sf *snowFlake) Generate() (int64, error) {
	return sf.node.Generate().Int64(), nil
}

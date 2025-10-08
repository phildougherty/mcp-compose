import TriggerNode from './TriggerNode';
import AITaskNode from './AITaskNode';
import MCPServerNode from './MCPServerNode';
import DecisionNode from './DecisionNode';
import TransformNode from './TransformNode';
import CodeNode from './CodeNode';

export const nodeTypes = {
  trigger: TriggerNode,
  'ai-task': AITaskNode,
  'mcp-server': MCPServerNode,
  decision: DecisionNode,
  transform: TransformNode,
  code: CodeNode,
};

export {
  TriggerNode,
  AITaskNode,
  MCPServerNode,
  DecisionNode,
  TransformNode,
  CodeNode,
};

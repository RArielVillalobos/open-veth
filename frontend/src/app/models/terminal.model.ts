export interface TerminalSession {
    nodeId: string;
    nodeName: string;
    status: 'connecting' | 'connected' | 'disconnected';
    title: string;
}

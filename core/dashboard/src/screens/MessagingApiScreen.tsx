import { useMemo } from 'react';
import { useStore } from '../store';
import { timeAgo, maskPhone } from '../utils';
import { MessageSquare, FileText, Image as ImageIcon, File, LayoutTemplate, MessageCircle } from 'lucide-react';
import { Card } from '../components/ui/card';
import type { GenericMessage } from '../types';

const TYPE_ICONS: Record<GenericMessage['type'], React.ReactNode> = {
  text: <FileText className="w-3 h-3" />,
  image: <ImageIcon className="w-3 h-3" />,
  document: <File className="w-3 h-3" />,
  template: <LayoutTemplate className="w-3 h-3" />
};

export default function MessagingApiScreen() {
  const genericMessages = useStore(state => state.genericMessages);

  const groupedMessages = useMemo(() => {
    const groups: Record<string, GenericMessage[]> = {};
    genericMessages.forEach(msg => {
      const to = msg.to || 'Unknown';
      if (!groups[to]) groups[to] = [];
      groups[to].push(msg);
    });
    // Sort conversations by latest message first
    return Object.entries(groups).sort((a, b) => {
      const latestA = new Date(a[1][0]?.sentAt || 0).getTime();
      const latestB = new Date(b[1][0]?.sentAt || 0).getTime();
      return latestB - latestA;
    });
  }, [genericMessages]);

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div className="flex flex-col gap-2 mb-8">
        <h2 className="text-3xl font-bold tracking-tight">Messaging API</h2>
        <p className="text-muted-foreground text-lg">Generic text and media messages sent via WhatsApp</p>
      </div>
      
      <Card className="p-6 bg-muted/30">
        <p className="text-muted-foreground">
          Use the Messaging API to send rich media, text messages, and templates to your users. 
          Currently viewing the latest outbound messages.
        </p>
      </Card>

      <Card className="p-4">
        {genericMessages.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground text-center">
            <MessageSquare className="w-12 h-12 mb-4 opacity-50" />
            <h3 className="text-lg font-semibold text-foreground mb-2">Waiting for events...</h3>
            <p className="max-w-sm">
              Send a message using the Messaging API to see it appear here.
            </p>
          </div>
        ) : (
          <div className="space-y-6">
            {groupedMessages.map(([to, msgs]) => (
              <div key={to} className="space-y-3">
                <div className="flex items-center gap-2 px-1">
                  <MessageCircle className="w-4 h-4 text-primary" />
                  <span className="font-semibold text-sm tracking-wide">{maskPhone(to)}</span>
                  <span className="text-xs text-muted-foreground ml-2">{msgs.length} messages</span>
                </div>
                
                <div className="flex flex-col gap-2">
                  {msgs.map((msg) => (
                    <div
                      key={msg.id}
                      className="flex items-center justify-between p-3 bg-muted/20 border border-border/50 rounded-lg hover:bg-muted/40 transition-colors gap-4 ml-2"
                    >
                      <div className="flex flex-col min-w-0">
                        <div className="flex items-center gap-2 text-sm text-foreground">
                          <span className="flex items-center gap-1.5 px-1.5 py-0.5 rounded bg-muted text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                            {TYPE_ICONS[msg.type]}
                            {msg.type}
                          </span>
                          <span className="truncate">{msg.content}</span>
                        </div>
                      </div>

                      <div className="flex items-center gap-3 shrink-0">
                        <span className={`text-[10px] font-medium uppercase tracking-wider px-2 py-0.5 rounded-full ${
                          msg.status === 'delivered' || msg.status === 'read' ? 'bg-[#25D366]/10 text-[#25D366]' :
                          msg.status === 'failed' ? 'bg-destructive/10 text-destructive' :
                          'bg-blue-500/10 text-blue-500'
                        }`}>
                          {msg.status}
                        </span>
                        <div className="text-[10px] text-muted-foreground whitespace-nowrap min-w-[50px] text-right font-mono">
                          {timeAgo(msg.sentAt)}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}

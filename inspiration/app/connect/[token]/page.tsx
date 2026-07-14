"use client";

import { AppLayout } from "@/components/AppLayout";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { AlertCircle, ArrowLeft, CheckCircle, Loader2, RefreshCw, ShieldCheck, Smartphone } from "lucide-react";
import { useRouter } from "next/navigation";
import { QRCodeSVG } from "qrcode.react";
import { use, useCallback, useEffect, useRef, useState } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8000";
const WS_URL = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8000";

type Status = "disconnected" | "connecting" | "connected" | "error";

export default function ConnectPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = use(params);
  const router = useRouter();
  
  const [qr, setQr] = useState<string | null>(null);
  const [status, setStatus] = useState<Status>("disconnected");
  const [log, setLog] = useState("Initialisation...");
  const [isRetrying, setIsRetrying] = useState(false);
  
  const wsRef = useRef<WebSocket | null>(null);
  const statusRef = useRef<Status>(status);

  // Keep statusRef in sync with status state to avoid stale closures in WS callbacks
  useEffect(() => {
    statusRef.current = status;
  }, [status]);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    setStatus("connecting");
    setLog("Connexion au serveur...");
    setQr(null);

    // Ensure we correct the protocol if needed (ws vs wss)
    let wsUrl = WS_URL;
    if (typeof window !== "undefined" && wsUrl.startsWith("/")) {
       wsUrl = `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}${WS_URL}`;
    }

    console.log("Connecting to WS:", `${wsUrl}/public/ws?token=${token}`);
    
    const ws = new WebSocket(`${wsUrl}/public/ws?token=${token}`);
    wsRef.current = ws;

    ws.onopen = () => {
      setLog("Attente du QR Code...");
      // Trigger WhatsApp connection once WebSocket is ready
      fetch(`${API_URL}/public/connect/${token}`, { method: "POST" })
        .then(res => {
            if (!res.ok) throw new Error("Erreur d'initiation");
            setLog("Génération du QR Code...");
        })
        .catch((err) => {
            console.error(err);
            setLog("Erreur lors de l'initiation de la session");
            setStatus("error");
        });
    };

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === "qr") {
            setQr(msg.qr);
            setStatus("disconnected"); 
            setLog("Scannez le QR Code");
        }
        if (msg.type === "status") {
            setStatus(msg.status);
            if (msg.status === "connected") {
                setLog("Connexion réussie !");
            } else if (msg.status === "connecting") {
                setLog("Authentification en cours...");
            }
        }
        if (msg.type === "log") setLog(msg.message);
        if (msg.type === "error") {
            setLog(msg.message);
            setStatus("error");
        }
      } catch (err) {
        console.error("Failed to parse WS message", err);
      }
    };

    ws.onerror = () => {
        console.error("WebSocket Error");
        setLog("Erreur de connexion WebSocket");
    };

    ws.onclose = () => {
        console.log("WebSocket Closed. Last status:", statusRef.current);
        if (statusRef.current !== "connected") {
            // Only show error/disconnect if we weren't successfully connected
        }
    };
  }, [token]);

  useEffect(() => {
    connect();
    return () => {
        if (wsRef.current) {
            wsRef.current.close();
            wsRef.current = null;
        }
    };
  }, [connect]);

  const handleReload = () => {
    setIsRetrying(true);
    // Force close and reconnect
    if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
    }
    // Small delay to allow cleanup
    setTimeout(() => {
        connect();
        setIsRetrying(false);
    }, 500);
  };

  return (
    <AppLayout showNewButton={false}>
      <div className="max-w-4xl mx-auto">
        <div className="flex items-center gap-3 mb-8">
            <Button
                variant="ghost"
                size="icon"
                onClick={() => router.push("/")}
                className="text-muted-foreground hover:text-foreground"
            >
                <ArrowLeft className="w-5 h-5" />
            </Button>
            <h1 className="text-2xl font-bold text-foreground">Connecter un compte WhatsApp</h1>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-8 items-start">
            <Card>
                <CardHeader>
                    <CardTitle className="text-xl flex items-center gap-2">
                        <Smartphone className="w-5 h-5 text-purple-500" />
                        Instructions
                    </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4 text-muted-foreground">
                    <div className="flex gap-3 items-start">
                        <div className="w-6 h-6 rounded-full bg-muted flex items-center justify-center text-sm font-medium text-foreground shrink-0">1</div>
                        <p>Ouvrez WhatsApp sur votre téléphone</p>
                    </div>
                    <div className="flex gap-3 items-start">
                        <div className="w-6 h-6 rounded-full bg-muted flex items-center justify-center text-sm font-medium text-foreground shrink-0">2</div>
                        <p>Allez dans <strong>Réglages</strong> {">"} <strong>Appareils connectés</strong></p>
                    </div>
                    <div className="flex gap-3 items-start">
                        <div className="w-6 h-6 rounded-full bg-muted flex items-center justify-center text-sm font-medium text-foreground shrink-0">3</div>
                        <p>Appuyez sur <strong>Connecter un appareil</strong></p>
                    </div>
                    <div className="flex gap-3 items-start">
                        <div className="w-6 h-6 rounded-full bg-muted flex items-center justify-center text-sm font-medium text-foreground shrink-0">4</div>
                        <p>Pointez votre téléphone vers cet écran pour scanner le code QR</p>
                    </div>

                    <div className="pt-4 mt-4 border-t border-border">
                        <div className="flex items-center gap-2 text-sm text-muted-foreground">
                            <ShieldCheck className="w-4 h-4 text-green-500" />
                            <span>Connexion chiffrée de bout en bout</span>
                        </div>
                    </div>
                </CardContent>
            </Card>

            <Card className="flex flex-col items-center justify-center min-h-[400px]">
                <CardContent className="flex flex-col items-center justify-center p-8 w-full">
                    {status === "connected" ? (
                        <div className="text-center space-y-4">
                            <div className="w-20 h-20 bg-green-500/10 rounded-full flex items-center justify-center mx-auto animate-in zoom-in duration-300">
                                <CheckCircle className="w-10 h-10 text-green-500" />
                            </div>
                            <h3 className="text-xl font-bold text-foreground">Connecté !</h3>
                            <p className="text-muted-foreground">Votre compte est prêt à être utilisé.</p>
                            <Button 
                                onClick={() => router.push("/")}
                                className="bg-[#25D366] hover:bg-[#20bd5a] text-black font-medium mt-4"
                            >
                                Retour au tableau de bord
                            </Button>
                        </div>
                    ) : status === "error" ? (
                        <div className="text-center space-y-4">
                            <div className="w-20 h-20 bg-destructive/10 rounded-full flex items-center justify-center mx-auto">
                                <AlertCircle className="w-10 h-10 text-destructive" />
                            </div>
                            <h3 className="text-xl font-bold text-destructive">Erreur de connexion</h3>
                            <p className="text-muted-foreground">{log}</p>
                            <Button variant="outline" onClick={handleReload} className="gap-2 border-border text-foreground hover:bg-muted">
                                <RefreshCw className={`w-4 h-4 ${isRetrying ? "animate-spin" : ""}`} />
                                Réessayer
                            </Button>
                        </div>
                    ) : (
                        <div className="flex flex-col items-center space-y-6">
                             <div className="relative group">
                                <div className={`p-4 bg-white rounded-xl transition-all duration-300 ${!qr ? 'blur-sm opacity-50' : ''}`}>
                                    {qr ? (
                                        <QRCodeSVG value={qr} size={256} level="L" />
                                    ) : (
                                        <div className="w-64 h-64 flex items-center justify-center">
                                            <Loader2 className="w-8 h-8 text-black/20 animate-spin" />
                                        </div>
                                    )}
                                </div>
                                {!qr && (
                                    <div className="absolute inset-0 flex items-center justify-center text-zinc-900 font-medium z-10">
                                        Chargement...
                                    </div>
                                )}
                            </div>

                            <div className="space-y-2 text-center">
                                <div className="flex items-center justify-center gap-2 text-foreground font-medium">
                                    {status === "connecting" && <Loader2 className="w-4 h-4 animate-spin" />}
                                    <span>{log}</span>
                                </div>
                                <p className="text-xs text-muted-foreground max-w-[250px] mx-auto">
                                    Gardez cette page ouverte pendant la synchronisation
                                </p>
                            </div>

                            <Button 
                                variant="ghost" 
                                size="sm" 
                                onClick={handleReload}
                                disabled={isRetrying}
                                className="text-muted-foreground hover:text-foreground text-xs gap-1"
                            >
                                <RefreshCw className={`w-3 h-3 ${isRetrying ? "animate-spin" : ""}`} />
                                Actualiser le code QR
                            </Button>
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
      </div>
    </AppLayout>
  );
}

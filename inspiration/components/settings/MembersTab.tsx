"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { authClient } from "@/lib/auth-client";
import { Loader2, Mail, Trash2, UserPlus } from "lucide-react";
import { useState } from "react";


export function MembersTab() {
    const { data: activeOrg, isPending, refetch } = authClient.useActiveOrganization();
    const members = activeOrg?.members || [];
    const { data: session } = authClient.useSession();
    
    console.log('[MembersTab] activeOrg:', activeOrg);
    console.log('[MembersTab] members:', members);
    console.log('[MembersTab] isPending:', isPending);
    
    // Invite State
    const [inviteEmail, setInviteEmail] = useState("");
    const [inviteRole, setInviteRole] = useState("member");
    const [inviting, setInviting] = useState(false);

    const handleInvite = async () => {
        if (!inviteEmail) return;
        setInviting(true);
        try {
            await authClient.organization.inviteMember({
                email: inviteEmail,
                role: inviteRole as "member" | "admin" | "owner",
            }, {
                onSuccess: () => {
                    setInviteEmail("");
                    // toast.success("Invitation envoyée !");
                    alert("Invitation envoyée !"); // Fallback
                    refetch();
                },
                onError: (ctx) => {
                    alert(ctx.error.message);
                }
            });
        } catch (e) {
            console.error(e);
        }
        setInviting(false);
    };

    const handleRemove = async (memberId: string) => {
        if (!confirm("Voulez-vous vraiment retirer ce membre ?")) return;
        await authClient.organization.removeMember({
            memberIdOrEmail: memberId,
        }, {
            onSuccess: () => refetch(),
            onError: (ctx) => alert(ctx.error.message)
        });
    }

    return (
        <div className="space-y-6 mt-4">
             {/* Invite Section */}
             <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <UserPlus className="w-5 h-5" />
                        Inviter un membre
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="flex gap-4">
                        <Input 
                            placeholder="email@example.com" 
                            className="bg-muted border-border text-foreground"
                            value={inviteEmail}
                            onChange={(e) => setInviteEmail(e.target.value)}
                        />
                        <select 
                            className="bg-muted border-border text-foreground rounded-md px-3 text-sm"
                            value={inviteRole}
                            onChange={(e) => setInviteRole(e.target.value)}
                        >
                            <option value="member">Membre</option>
                            <option value="admin">Administrateur</option>
                        </select>
                        <Button 
                            onClick={handleInvite} 
                            disabled={inviting || !inviteEmail}
                            className="bg-[#25D366] text-white hover:bg-[#1ebe5d]"
                        >
                            {inviting ? <Loader2 className="w-4 h-4 animate-spin" /> : "Inviter"}
                        </Button>
                    </div>
                </CardContent>
            </Card>

            {/* List Section */}
            <Card>
                <CardHeader>
                    <CardTitle>Membres de l'équipe</CardTitle>
                </CardHeader>
                <CardContent>
                    {isPending ? (
                        <div className="flex justify-center p-4">
                            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
                            <span className="ml-2 text-muted-foreground">Chargement des membres...</span>
                        </div>
                    ) : !activeOrg ? (
                        <p className="text-muted-foreground text-center py-4">Aucune organisation active sélectionnée.</p>
                    ) : (
                        <div className="space-y-4">
                            {members?.map((member) => (
                                <div key={member.id} className="flex items-center justify-between p-3 rounded-lg bg-muted/30 border border-border">
                                    <div className="flex items-center gap-3">
                                        <div className="w-10 h-10 rounded-full bg-muted flex items-center justify-center text-muted-foreground font-medium border border-border">
                                            {member.user.name?.charAt(0).toUpperCase()}
                                        </div>
                                        <div>
                                            <p className="text-foreground font-medium">{member.user.name}</p>
                                            <p className="text-muted-foreground text-xs flex items-center gap-1">
                                                <Mail className="w-3 h-3" />
                                                {member.user.email}
                                            </p>
                                        </div>
                                    </div>
                                    <div className="flex items-center gap-4">
                                        <span className={`text-xs px-2 py-1 rounded-full ${
                                            member.role === "owner" ? "bg-purple-500/10 text-purple-500" :
                                            member.role === "admin" ? "bg-blue-500/10 text-blue-500" :
                                            "bg-muted text-muted-foreground"
                                        }`}>
                                            {member.role.toUpperCase()}
                                        </span>
                                        
                                        {/* Actions: Can't remove yourself, only admin/owner can remove */}
                                        {session?.user.id !== member.userId && (
                                            <Button 
                                                variant="ghost" 
                                                size="icon" 
                                                className="text-destructive hover:text-destructive hover:bg-destructive/10"
                                                onClick={() => handleRemove(member.id)}
                                            >
                                                <Trash2 className="w-4 h-4" />
                                            </Button>
                                        )}
                                    </div>
                                </div>
                            ))}
                            {members?.length === 0 && (
                                <p className="text-muted-foreground text-center py-4">Aucun membre trouvé.</p>
                            )}
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}

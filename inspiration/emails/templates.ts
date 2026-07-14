export type InvitationEmailProps = {
  email: string;
  senderName: string;
  organizationName: string;
  inviteLink: string;
};

export type PasswordResetEmailProps = {
  resetLink: string;
};

export type SessionNotificationProps = {
  sessionDate: string;
  clientName: string;
};

export type InvoiceNotificationProps = {
  invoiceId: string;
  amount: string;
};

export type WelcomeEmailProps = {
  name: string;
};

export function invitationTemplate({ senderName, organizationName, inviteLink }: InvitationEmailProps) {
  return {
    subject: `Invitation à rejoindre ${organizationName}`,
    html: `<p>${senderName} vous a invité à rejoindre ${organizationName}.</p>
           <p><a href="${inviteLink}">Cliquez ici pour accepter l'invitation</a></p>`
  };
}

export function passwordResetTemplate({ resetLink }: PasswordResetEmailProps) {
  return {
    subject: "Réinitialisation de votre mot de passe",
    html: `<p>Vous avez demandé la réinitialisation de votre mot de passe.</p>
           <p><a href="${resetLink}">Cliquez ici pour définir un nouveau mot de passe</a></p>
           <p>Si vous n'êtes pas à l'origine de cette demande, ignorez cet email.</p>`
  };
}

export function sessionNotificationTemplate({ sessionDate, clientName }: SessionNotificationProps) {
  return {
    subject: "Notification de session",
    html: `<p>Session prévue le ${sessionDate} avec ${clientName}.</p>`
  };
}

export function invoiceNotificationTemplate(props: InvoiceNotificationProps) {
  return {
    subject: "Nouvelle facture",
    html: `<p>Facture ${props.invoiceId} de ${props.amount} disponible.</p>`
  };
}

export function welcomeTemplate({ name }: WelcomeEmailProps) {
  return {
    subject: "Bienvenue !",
    html: `<p>Bienvenue ${name} sur notre plateforme.</p>`
  };
}

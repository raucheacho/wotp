import React from "react";

const StatCard = ({
  value,
  icon,
  title,
}: {
  value: string | number;
  icon: React.ReactNode;
  title: string;
}) => {
  return (
    <div className="bg-card border border-border rounded-xl p-4">
      <div className="flex items-start flex-col gap-3 justify-center">
        <div className="flex items-center gap-2">
          {icon}
          <p className="text-xl font-semibold text-card-foreground pl-2">{value}</p>
        </div>
        <p className="text-xs text-muted-foreground">{title}</p>
      </div>
    </div>
  );
};

export default StatCard;

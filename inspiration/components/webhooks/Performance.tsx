const Performance = ({ successRate }: { successRate: number }) => {
  return (
    <div className="mb-6">
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm text-zinc-400">Taux de succès</span>
        <span
          className={`text-sm font-medium ${
            successRate >= 95
              ? "text-green-400"
              : successRate >= 80
                ? "text-yellow-400"
                : "text-red-400"
          }`}
        >
          {successRate.toFixed(1)}%
        </span>
      </div>
      <div className="w-full bg-zinc-700 rounded-full h-2">
        <div
          className={`h-2 rounded-full transition-all duration-300 ${
            successRate >= 95
              ? "bg-green-500"
              : successRate >= 80
                ? "bg-yellow-500"
                : "bg-red-500"
          }`}
          style={{ width: `${Math.min(successRate, 100)}%` }}
        />
      </div>
      <div className="flex justify-between text-xs text-zinc-600 mt-1">
        <span>0%</span>
        <span>50%</span>
        <span>100%</span>
      </div>
    </div>
  );
};

export default Performance;

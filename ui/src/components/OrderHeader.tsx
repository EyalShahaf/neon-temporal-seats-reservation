import React from 'react';

interface OrderHeaderProps {
  orderId: string;
  status: string;
}

const statusColors: { [key: string]: string } = {
  PENDING: 'bg-cyan-500/20 text-cyan-400 border-cyan-500/50',
  SEATS_SELECTED: 'bg-blue-500/20 text-blue-400 border-blue-500/50',
  CONFIRMED: 'bg-green-500/20 text-green-400 border-green-500/50',
  FAILED: 'bg-red-500/20 text-red-400 border-red-500/50',
  EXPIRED: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50',
};

const OrderHeader: React.FC<OrderHeaderProps> = ({ orderId, status }) => {
  const color = statusColors[status] || 'bg-gray-500/20 text-gray-400 border-gray-500/50';

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-lg font-bold text-cyan-400 font-mono">Order Details</h2>
          <p className="text-gray-300 font-mono text-sm">ID: {orderId}</p>
        </div>
        <div className="text-right">
          <span className={`px-4 py-2 text-sm font-semibold rounded-lg border ${color} font-mono`}>
            {status.replace(/_/g, ' ')}
          </span>
        </div>
      </div>
    </div>
  );
};

export default OrderHeader;

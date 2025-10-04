import React, { useState } from 'react';

interface PaymentFormProps {
  attemptsLeft: number;
  lastError: string;
  isLocked: boolean;
  onSubmit: (paymentCode: string) => void;
}

const PaymentForm: React.FC<PaymentFormProps> = ({
  attemptsLeft,
  lastError,
  isLocked,
  onSubmit,
}) => {
  const [code, setCode] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (code.length === 5 && !isLocked) {
      onSubmit(code);
      setCode(''); // Clear the input after submission
    }
  };

  return (
    <form className="space-y-6" onSubmit={handleSubmit}>
      <h3 className="text-xl font-bold text-cyan-400 font-mono">Payment</h3>
      
      <div>
        <label htmlFor="payment-code" className="block text-sm font-medium text-gray-300 font-mono mb-2">
          Payment Code ({attemptsLeft} attempts left)
        </label>
        <input
          type="text"
          id="payment-code"
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 5))}
          maxLength={5}
          disabled={isLocked}
          placeholder="Enter 5-digit code"
          className="w-full bg-gray-700 border-2 border-gray-600 rounded-lg shadow-sm py-3 px-4 text-white font-mono text-lg text-center focus:outline-none focus:ring-2 focus:ring-cyan-500/50 focus:border-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
        />
      </div>

      {lastError && (
        <div className="bg-red-500/20 border border-red-500/50 rounded-lg p-3">
          <p className="text-sm text-red-400 font-mono">Error: {lastError}</p>
        </div>
      )}

      <button
        type="submit"
        disabled={isLocked || code.length !== 5}
        className="w-full bg-cyan-500/20 border-2 border-cyan-500 text-cyan-400 font-bold py-3 px-4 rounded-lg font-mono hover:bg-cyan-500/30 hover:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/50 disabled:bg-gray-700 disabled:border-gray-600 disabled:text-gray-500 disabled:cursor-not-allowed transition-all duration-200"
      >
        {isLocked ? 'Payment Locked' : 'Submit Payment'}
      </button>
    </form>
  );
};

export default PaymentForm;

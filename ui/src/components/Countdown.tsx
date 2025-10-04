import React, { useState, useEffect } from 'react';

interface CountdownProps {
  expiresAt: string | null;
}

const Countdown: React.FC<CountdownProps> = ({ expiresAt }) => {
  const [remaining, setRemaining] = useState('');

  useEffect(() => {
    if (!expiresAt) {
      setRemaining('--:--');
      return;
    }

    // Update immediately on mount
    const updateTime = () => {
      const now = new Date();
      const expiry = new Date(expiresAt);
      const diff = expiry.getTime() - now.getTime();

      if (diff <= 0) {
        setRemaining('00:00');
        return;
      }

      const minutes = Math.floor(diff / (1000 * 60));
      const seconds = Math.floor((diff % (1000 * 60)) / 1000);

      const timeString = `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
      setRemaining(timeString);
    };

    // Update immediately
    updateTime();

    // Then update every second
    const interval = setInterval(updateTime, 1000);

    return () => clearInterval(interval);
  }, [expiresAt]);

  const isExpired = remaining === '00:00';
  const isLowTime = remaining !== '--:--' && remaining !== '00:00' && parseInt(remaining.split(':')[0]) < 5;

  return (
    <div className="text-center space-y-4">
      <h4 className="text-lg font-bold text-cyan-400 font-mono">Hold Timer</h4>
      <div className={`
        p-6 rounded-lg border-2 transition-all duration-200
        ${isExpired 
          ? 'bg-red-500/20 border-red-500 text-red-400' 
          : isLowTime 
            ? 'bg-yellow-500/20 border-yellow-500 text-yellow-400 animate-pulse' 
            : 'bg-cyan-500/20 border-cyan-500 text-cyan-400'
        }
      `}>
        <p className="text-5xl font-mono font-bold">{remaining}</p>
        <p className="text-sm font-mono mt-2 opacity-75">
          {isExpired ? 'Seat hold expired' : isLowTime ? 'Time running low!' : 'Seats reserved'}
        </p>
      </div>
    </div>
  );
};

export default Countdown;

import React, { useState, useEffect } from 'react';
import clsx from 'clsx';

interface SeatGridProps {
  selectedSeats: string[];
  onSeatsChanged: (seats: string[]) => void;
  isLocked: boolean;
  flightID?: string; // NEW - for fetching seat availability
}

interface SeatAvailability {
  available: string[];
  held: string[];
  confirmed: string[];
}

const SEAT_ROWS = 5;
const SEAT_COLS = 6;

const SeatGrid: React.FC<SeatGridProps> = ({ selectedSeats, onSeatsChanged, isLocked, flightID }) => {
  const [localSelection, setLocalSelection] = useState<string[]>(selectedSeats);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [lastClickedSeat, setLastClickedSeat] = useState<{ seatId: string; time: number } | null>(null);
  const [isConfirming, setIsConfirming] = useState(false);
  const [seatAvailability, setSeatAvailability] = useState<SeatAvailability>({ available: [], held: [], confirmed: [] });

  // Fetch seat availability when flightID changes
  useEffect(() => {
    if (!flightID) return;
    
    const fetchAvailability = async () => {
      try {
        const response = await fetch(`/flights/${flightID}/available-seats`);
        if (response.ok) {
          const data = await response.json();
          setSeatAvailability(data);
        }
      } catch (error) {
        console.error('Failed to fetch seat availability:', error);
      }
    };
    
    fetchAvailability();
    // Refresh availability every 1 second for real-time testing
    const interval = setInterval(fetchAvailability, 1000);
    return () => clearInterval(interval);
  }, [flightID]);

  useEffect(() => {
    // Only sync from parent if we're in confirming state OR if local matches parent
    // This prevents overwriting user's local selection before they confirm
    const localSorted = [...localSelection].sort().join(',');
    const parentSorted = [...selectedSeats].sort().join(',');
    
    if (isConfirming || localSorted === parentSorted) {
      setLocalSelection(selectedSeats);
      if (isConfirming) {
        setIsConfirming(false);
      }
    }
  }, [selectedSeats, isConfirming, localSelection]);

  const handleSeatClick = (seatId: string) => {
    if (isLocked || isConfirming) return;
    
    // Check if seat is unavailable (held or confirmed by others)
    const isSeatUnavailable = seatAvailability.held.includes(seatId) || seatAvailability.confirmed.includes(seatId);
    if (isSeatUnavailable) {
      return; // Don't allow clicking on unavailable seats
    }
    
    // Prevent rapid clicking on the SAME seat (debounce per seat)
    const now = Date.now();
    if (lastClickedSeat && lastClickedSeat.seatId === seatId && now - lastClickedSeat.time < 300) {
      return; // Ignore rapid clicks on the same seat
    }
    setLastClickedSeat({ seatId, time: now });
    
    setLocalSelection((prev) =>
      prev.includes(seatId) ? prev.filter((s) => s !== seatId) : [...prev, seatId]
    );
  };

  const handleConfirmSelection = async () => {
    if (isSubmitting || isConfirming) return;
    setIsSubmitting(true);
    setIsConfirming(true);
    try {
      await onSeatsChanged(localSelection);
    } catch (error) {
      // Reset confirming state on error
      setIsConfirming(false);
      throw error;
    } finally {
      setIsSubmitting(false);
      // Keep isConfirming true until parent state updates
    }
  };

  const hasChanges =
    JSON.stringify(localSelection.sort()) !== JSON.stringify(selectedSeats.sort());

  const getSeatState = (seatId: string) => {
    if (seatAvailability.confirmed.includes(seatId)) return 'confirmed';
    if (seatAvailability.held.includes(seatId)) return 'held';
    if (localSelection.includes(seatId)) return 'selected';
    return 'available';
  };

  const seats = [];
  for (let row = 1; row <= SEAT_ROWS; row++) {
    for (let col = 1; col <= SEAT_COLS; col++) {
      const seatId = `${row}${String.fromCharCode(64 + col)}`;
      const seatState = getSeatState(seatId);
      const isLocallySelected = localSelection.includes(seatId);
      const isConfirmed = selectedSeats.includes(seatId);
      const isBeingConfirmed = isConfirming && isLocallySelected;

      seats.push(
        <div
          key={seatId}
          onClick={() => handleSeatClick(seatId)}
          className={clsx(
            'w-14 h-14 rounded-lg flex items-center justify-center font-bold text-sm select-none transition-all duration-200 border-2 relative',
            {
              // Available seats
              'bg-gray-700 hover:bg-gray-600 cursor-pointer border-gray-600 text-gray-300 hover:text-white hover:border-cyan-500/50': seatState === 'available' && !isLocallySelected && !isConfirmed && !isLocked && !isConfirming,
              // Selected seats
              'bg-cyan-500/20 border-cyan-500 text-cyan-400 cursor-pointer hover:bg-cyan-500/30 shadow-lg shadow-cyan-500/25': isLocallySelected && !isBeingConfirmed,
              // Being confirmed
              'bg-orange-500/20 border-orange-500 text-orange-400 cursor-not-allowed shadow-lg shadow-orange-500/25': isBeingConfirmed,
              // Confirmed by this order
              'bg-green-500/20 border-green-500 text-green-400': isConfirmed,
              // Held by others (yellow)
              'bg-yellow-500/20 border-yellow-500 text-yellow-400 cursor-not-allowed': seatState === 'held',
              // Confirmed by others (red)
              'bg-red-500/20 border-red-500 text-red-400 cursor-not-allowed': seatState === 'confirmed',
              // Disabled states
              'bg-gray-800 text-gray-500 cursor-not-allowed border-gray-700': (isLocked || isConfirming) && !isLocallySelected && !isConfirmed,
            }
          )}
        >
          {seatId}
          {isLocallySelected && hasChanges && !isBeingConfirmed && (
            <div className="absolute -top-1 -right-1 w-3 h-3 bg-cyan-400 rounded-full animate-pulse"></div>
          )}
          {isBeingConfirmed && (
            <div className="absolute -top-1 -right-1 w-3 h-3 bg-orange-400 rounded-full animate-pulse"></div>
          )}
          {seatState === 'held' && (
            <div className="absolute -top-1 -right-1 w-3 h-3 bg-yellow-400 rounded-full"></div>
          )}
          {seatState === 'confirmed' && (
            <div className="absolute -top-1 -right-1 w-3 h-3 bg-red-400 rounded-full"></div>
          )}
        </div>
      );
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h3 className="text-xl font-bold text-cyan-400 font-mono">Seat Selection</h3>
        <div className="text-sm text-gray-400 font-mono">
          {localSelection.length} seat{localSelection.length !== 1 ? 's' : ''} selected
        </div>
      </div>
      
      <div className="grid grid-cols-6 gap-3 justify-items-center">
        {seats}
      </div>
      
      {/* Seat Legend */}
      <div className="flex flex-wrap gap-4 text-xs text-gray-400 justify-center">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-gray-700 border border-gray-600 rounded"></div>
          <span>Available</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-cyan-500/20 border border-cyan-500 rounded"></div>
          <span>Selected</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-yellow-500/20 border border-yellow-500 rounded"></div>
          <span>Held by others</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-red-500/20 border border-red-500 rounded"></div>
          <span>Confirmed by others</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-green-500/20 border border-green-500 rounded"></div>
          <span>Your confirmed</span>
        </div>
      </div>
      
      {hasChanges && !isLocked && (
        <div className="flex justify-center pt-4">
          <button
            onClick={handleConfirmSelection}
            disabled={isSubmitting || isConfirming || localSelection.length === 0}
            className={clsx(
              'px-6 py-3 rounded-lg font-bold font-mono transition-all duration-200 border-2',
              {
                'bg-cyan-500/20 border-cyan-500 text-cyan-400 hover:bg-cyan-500/30 hover:border-cyan-400 cursor-pointer': !isSubmitting && !isConfirming && localSelection.length > 0,
                'bg-orange-500/20 border-orange-500 text-orange-400 cursor-not-allowed': isConfirming,
                'bg-gray-700 border-gray-600 text-gray-500 cursor-not-allowed': isSubmitting || localSelection.length === 0,
              }
            )}
          >
            {isConfirming ? 'Confirming...' : isSubmitting ? 'Processing...' : `Confirm ${localSelection.length} Seat${localSelection.length !== 1 ? 's' : ''}`}
          </button>
        </div>
      )}
      
      {isConfirming && (
        <div className="text-center text-orange-400 font-mono text-sm">
          🔄 Confirming seat selection...
        </div>
      )}
      
      {isLocked && !isConfirming && (
        <div className="text-center text-gray-400 font-mono text-sm">
          Seat selection is locked
        </div>
      )}
    </div>
  );
};

export default SeatGrid;

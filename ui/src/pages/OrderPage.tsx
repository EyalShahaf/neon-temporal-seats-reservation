import React, { useState, useCallback } from 'react';
import OrderHeader from '../components/OrderHeader';
import SeatGrid from '../components/SeatGrid';
import Countdown from '../components/Countdown';
import PaymentForm from '../components/PaymentForm';
import { useEventSource } from '../hooks/useEventSource';

// Matches the Go backend's workflows.OrderState
interface OrderState {
  State: string;
  Seats: string[];
  HoldExpiresAt: string; // ISO 8601 string
  AttemptsLeft: number;
  LastPaymentErr: string;
  PaymentStatus?: string; // NEW: trying, retrying, failed, success
}

const API_BASE_URL = 'http://localhost:8080';
const FLIGHT_ID = 'FL-001';

const OrderPage: React.FC = () => {
  // Generate a unique order ID for this session
  const [orderId] = useState(() => `order-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`);
  const [isCreating, setIsCreating] = useState(true);
  const [orderCreated, setOrderCreated] = useState(false);

  // Only start SSE connection AFTER order is created
  const { data: orderState } = useEventSource<OrderState>(
    orderCreated ? `${API_BASE_URL}/orders/${orderId}/events` : ''
  );

  // Effect to create the order once on component mount
  React.useEffect(() => {
    const createOrder = async () => {
      try {
        await fetch(`${API_BASE_URL}/orders`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ orderID: orderId, flightID: FLIGHT_ID }),
        });
        // Once the POST is successful, we can start listening for events
        setOrderCreated(true);
      } catch (err) {
        console.error('Failed to create order:', err);
        // Handle error state appropriately in a real app
      } finally {
        setIsCreating(false);
      }
    };
    createOrder();
  }, []);

  const handleSeatsChanged = useCallback(
    async (newSeats: string[]) => {
      console.log('New seats selected:', newSeats);
      try {
        await fetch(`${API_BASE_URL}/orders/${orderId}/seats`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ seats: newSeats }),
        });
      } catch (err) {
        console.error('Failed to update seats:', err);
      }
    },
    [orderId]
  );

  const handlePaymentSubmit = useCallback(async (paymentCode: string) => {
    console.log('Submitting payment with code:', paymentCode);
    try {
      await fetch(`${API_BASE_URL}/orders/${orderId}/payment`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: paymentCode }),
      });
    } catch (err) {
      console.error('Failed to submit payment:', err);
    }
  }, [orderId]);

  if (isCreating) {
    return (
      <div className="bg-gray-900 min-h-screen flex items-center justify-center">
        <div className="text-cyan-400 text-xl font-mono animate-pulse">
          🚀 Creating order {orderId}...
        </div>
      </div>
    );
  }
  
  if (!orderState && orderCreated) {
    return (
      <div className="bg-gray-900 min-h-screen flex items-center justify-center">
        <div className="text-cyan-400 text-xl font-mono animate-pulse">
          📡 Connecting to order stream...
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-900 min-h-screen p-8">
      <div className="max-w-6xl mx-auto space-y-6">
        {/* Order Info Header */}
        <div className="bg-gray-800 rounded-lg shadow-2xl border border-cyan-500/20 p-6">
          <div className="flex items-center justify-between mb-4">
            <h1 className="text-2xl font-bold text-cyan-400 font-mono">Temporal Seats</h1>
            <div className="text-sm text-gray-400 font-mono">
              Flight: {FLIGHT_ID}
            </div>
          </div>
          <OrderHeader orderId={orderId} status={orderState.State} />
        </div>
        {/* Seat Selection */}
        <div className="bg-gray-800 rounded-lg shadow-2xl border border-cyan-500/20 p-6">
          <SeatGrid
            selectedSeats={orderState.Seats}
            onSeatsChanged={handleSeatsChanged}
            isLocked={orderState.State !== 'PENDING' && orderState.State !== 'SEATS_SELECTED'}
            flightID={FLIGHT_ID}
            currentOrderSeats={orderState.Seats}
          />
        </div>

        {/* Timer and Payment */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="bg-gray-800 rounded-lg shadow-2xl border border-cyan-500/20 p-6">
            <Countdown expiresAt={orderState.HoldExpiresAt} />
          </div>
          <div className="bg-gray-800 rounded-lg shadow-2xl border border-cyan-500/20 p-6">
            <PaymentForm
              attemptsLeft={orderState.AttemptsLeft}
              lastError={orderState.LastPaymentErr}
              paymentStatus={orderState.PaymentStatus}
              onSubmit={handlePaymentSubmit}
              isLocked={orderState.State !== 'SEATS_SELECTED'}
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default OrderPage;

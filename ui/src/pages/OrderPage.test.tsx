import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, beforeAll } from 'vitest';
import * as useEventSourceModule from '../hooks/useEventSource';

// Mock the useEventSource hook
vi.mock('../hooks/useEventSource');

// Mock fetch
global.fetch = vi.fn();

describe('OrderPage', () => {
  const mockUseEventSource = vi.mocked(useEventSourceModule.useEventSource);
  
  // Lazy import OrderPage after mocking
  let OrderPage: any;
  
  beforeAll(async () => {
    OrderPage = (await import('./OrderPage')).default;
  });
  
  const mockOrderState = {
    State: 'PENDING',
    Seats: [],
    HoldExpiresAt: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
    AttemptsLeft: 3,
    LastPaymentErr: '',
  };

  // Helper function to create proper mock return value
  const createMockReturnValue = (data: any) => ({
    data,
    readyState: 1, // EventSource.OPEN
    close: vi.fn()
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseEventSource.mockReturnValue(createMockReturnValue(mockOrderState));
    (global.fetch as any).mockResolvedValue({ ok: true });
  });

  it('renders loading state initially', () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue(null));
    
    render(<OrderPage />);
    
    expect(screen.getByText(/Creating order/)).toBeInTheDocument();
  });

  it('renders order page with seat selection and payment form', async () => {
    render(<OrderPage />);
    
    // Wait for the order creation to complete
    await waitFor(() => {
      expect(screen.getByText('Temporal Seats')).toBeInTheDocument();
    });
    
    expect(screen.getByText('Seat Selection')).toBeInTheDocument();
    expect(screen.getByText('Payment')).toBeInTheDocument();
    expect(screen.getByText('0 seats selected')).toBeInTheDocument();
  });

  it('allows seat selection when state is PENDING', () => {
    render(<OrderPage />);
    
    const seat1A = screen.getByText('1A');
    fireEvent.click(seat1A);
    
    expect(seat1A).toHaveClass('bg-cyan-500/20');
    expect(screen.getByText('1 seat selected')).toBeInTheDocument();
  });

  it('shows confirm button when seats are selected', () => {
    render(<OrderPage />);
    
    fireEvent.click(screen.getByText('1A'));
    expect(screen.getByText('Confirm 1 Seat')).toBeInTheDocument();
  });

  it('calls API when seats are confirmed', async () => {
    render(<OrderPage />);
    
    fireEvent.click(screen.getByText('1A'));
    fireEvent.click(screen.getByText('Confirm 1 Seat'));
    
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/seats'),
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: expect.stringContaining('"seats":["1A"]'),
        })
      );
    });
  });

  it('allows payment when seats are selected', () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, State: 'SEATS_SELECTED', Seats: ['1A']
    }));
    
    render(<OrderPage />);
    
    const paymentInput = screen.getByLabelText(/payment code/i);
    const paymentButton = screen.getByRole('button', { name: /submit payment/i });
    
    expect(paymentInput).not.toBeDisabled();
    expect(paymentButton).not.toBeDisabled();
  });

  it('calls payment API when payment is submitted', async () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, State: 'SEATS_SELECTED', Seats: ['1A']
    }));
    
    render(<OrderPage />);
    
    const paymentInput = screen.getByLabelText(/payment code/i);
    const paymentButton = screen.getByRole('button', { name: /submit payment/i });
    
    fireEvent.change(paymentInput, { target: { value: '12345' } });
    fireEvent.click(paymentButton);
    
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/payment'),
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: expect.stringContaining('"code":"12345"'),
        })
      );
    });
  });

  it('locks seat selection when state is not PENDING or SEATS_SELECTED', () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, State: 'PAYMENT_PENDING'
    }));
    
    render(<OrderPage />);
    
    const seat1A = screen.getByText('1A');
    fireEvent.click(seat1A);
    
    expect(seat1A).not.toHaveClass('bg-cyan-500/20');
    expect(seat1A).toHaveClass('cursor-not-allowed');
  });

  it('locks payment when state is not SEATS_SELECTED', () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, State: 'PENDING'
    }));
    
    render(<OrderPage />);
    
    const paymentInput = screen.getByLabelText(/payment code/i);
    const paymentButton = screen.getByRole('button', { name: /submit payment/i });
    
    expect(paymentInput).toBeDisabled();
    expect(paymentButton).toBeDisabled();
  });

  it('shows payment error when provided', () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, LastPaymentErr: 'Invalid payment code'
    }));
    
    render(<OrderPage />);
    
    expect(screen.getByText(/Error: Invalid payment code/)).toBeInTheDocument();
  });

  it('shows attempts left in payment form', () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, AttemptsLeft: 2
    }));
    
    render(<OrderPage />);
    
    expect(screen.getByText(/2 attempts left/)).toBeInTheDocument();
  });

  it('shows confirmed seats with green styling', () => {
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, State: 'SEATS_SELECTED', Seats: ['1A', '2B']
    }));
    
    render(<OrderPage />);
    
    const seat1A = screen.getByText('1A');
    const seat2B = screen.getByText('2B');
    
    expect(seat1A).toHaveClass('bg-green-500/20');
    expect(seat2B).toHaveClass('bg-green-500/20');
  });

  it('handles seat confirmation with visual feedback', async () => {
    render(<OrderPage />);
    
    // Select seats
    fireEvent.click(screen.getByText('1A'));
    fireEvent.click(screen.getByText('2B'));
    
    // Start confirmation
    fireEvent.click(screen.getByText('Confirm 2 Seats'));
    
    // Should show confirming state
    expect(screen.getByText('1A')).toHaveClass('bg-orange-500/20');
    expect(screen.getByText('2B')).toHaveClass('bg-orange-500/20');
    expect(screen.getByText('🔄 Confirming seat selection...')).toBeInTheDocument();
    
    // Wait for API call
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/seats'),
        expect.objectContaining({
          method: 'POST',
          body: expect.stringContaining('"seats":["1A","2B"]'),
        })
      );
    });
  });

  it('handles network errors gracefully', async () => {
    (global.fetch as any).mockRejectedValue(new Error('Network error'));
    
    render(<OrderPage />);
    
    fireEvent.click(screen.getByText('1A'));
    fireEvent.click(screen.getByText('Confirm 1 Seat'));
    
    // Should handle error without crashing
    await waitFor(() => {
      expect(screen.getByText('1A')).toHaveClass('bg-cyan-500/20'); // Back to local selection
    });
  });

  it('maintains state consistency during rapid updates', () => {
    const { rerender } = render(<OrderPage />);
    
    // Select seats
    fireEvent.click(screen.getByText('1A'));
    expect(screen.getByText('1A')).toHaveClass('bg-cyan-500/20');
    
    // Simulate rapid state updates
    mockUseEventSource.mockReturnValue(createMockReturnValue({
      ...mockOrderState, State: 'SEATS_SELECTED', Seats: ['1A']
    }));
    rerender(<OrderPage />);
    
    // Should show confirmed state
    expect(screen.getByText('1A')).toHaveClass('bg-green-500/20');
  });
});

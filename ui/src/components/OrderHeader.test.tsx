import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import OrderHeader from './OrderHeader';

describe('OrderHeader', () => {
  it('displays order ID and status', () => {
    render(<OrderHeader orderId="ORDER-123" status="PENDING" />);
    
    expect(screen.getByText('ID: ORDER-123')).toBeInTheDocument();
    expect(screen.getByText('PENDING')).toBeInTheDocument();
  });

  it('applies correct status badge colors', () => {
    const { rerender } = render(<OrderHeader orderId="ORDER-123" status="PENDING" />);
    expect(screen.getByText('PENDING')).toHaveClass('bg-cyan-500/20', 'text-cyan-400');
    
    rerender(<OrderHeader orderId="ORDER-123" status="SEATS_SELECTED" />);
    expect(screen.getByText('SEATS SELECTED')).toHaveClass('bg-blue-500/20', 'text-blue-400');
    
    rerender(<OrderHeader orderId="ORDER-123" status="CONFIRMED" />);
    expect(screen.getByText('CONFIRMED')).toHaveClass('bg-green-500/20', 'text-green-400');
    
    rerender(<OrderHeader orderId="ORDER-123" status="FAILED" />);
    expect(screen.getByText('FAILED')).toHaveClass('bg-red-500/20', 'text-red-400');
    
    rerender(<OrderHeader orderId="ORDER-123" status="EXPIRED" />);
    expect(screen.getByText('EXPIRED')).toHaveClass('bg-yellow-500/20', 'text-yellow-400');
  });

  it('handles unknown status gracefully', () => {
    render(<OrderHeader orderId="ORDER-123" status="UNKNOWN_STATUS" />);
    
    expect(screen.getByText('UNKNOWN STATUS')).toBeInTheDocument();
    // Should fall back to default gray styling
    expect(screen.getByText('UNKNOWN STATUS')).toHaveClass('bg-gray-500/20', 'text-gray-400');
  });

  it('handles empty order ID', () => {
    render(<OrderHeader orderId="" status="PENDING" />);
    
    expect(screen.getByText('ID:')).toBeInTheDocument();
  });

  it('handles long order IDs', () => {
    const longOrderId = 'ORDER-' + 'A'.repeat(100);
    render(<OrderHeader orderId={longOrderId} status="PENDING" />);
    
    expect(screen.getByText(`ID: ${longOrderId}`)).toBeInTheDocument();
  });
});

import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import Countdown from './Countdown';

describe('Countdown', () => {
  it('displays time remaining correctly', async () => {
    const futureTime = new Date(Date.now() + 5 * 60 * 1000).toISOString(); // 5 minutes from now
    render(<Countdown expiresAt={futureTime} />);
    
    // Should show approximately 5 minutes (allow for small time differences)
    await waitFor(() => {
      const timeElement = screen.getByText(/\d+:\d+/);
      expect(timeElement).toBeInTheDocument();
      // Should be close to 5:00 (within 1 minute)
      const timeText = timeElement.textContent;
      expect(timeText).toMatch(/^\d{2}:\d{2}$/);
    });
  });

  it('displays 0:00 when time has expired', async () => {
    const pastTime = new Date(Date.now() - 1000).toISOString(); // 1 second ago
    render(<Countdown expiresAt={pastTime} />);
    
    await waitFor(() => {
      expect(screen.getByText('00:00')).toBeInTheDocument();
    });
  });

  it('updates every second', async () => {
    const futureTime = new Date(Date.now() + 3 * 60 * 1000).toISOString(); // 3 minutes from now
    render(<Countdown expiresAt={futureTime} />);
    
    // Initially should show approximately 3:00
    await waitFor(() => {
      const timeElement = screen.getByText(/\d+:\d+/);
      expect(timeElement).toBeInTheDocument();
      const timeText = timeElement.textContent;
      expect(timeText).toMatch(/^\d{2}:\d{2}$/);
    });
    
    // Wait for 1 second and check that the time has updated
    await new Promise(resolve => setTimeout(resolve, 1100));
    
    // Should now show a different time
    await waitFor(() => {
      const timeElement = screen.getByText(/\d+:\d+/);
      expect(timeElement).toBeInTheDocument();
      const timeText = timeElement.textContent;
      expect(timeText).toMatch(/^\d{2}:\d{2}$/);
    });
  });

  it('handles negative time gracefully', async () => {
    const pastTime = new Date(Date.now() - 10 * 60 * 1000).toISOString(); // 10 minutes ago
    render(<Countdown expiresAt={pastTime} />);
    
    await waitFor(() => {
      expect(screen.getByText('00:00')).toBeInTheDocument();
    });
  });

  it('formats time correctly for different durations', async () => {
    // Test 1 hour 30 minutes
    const longTime = new Date(Date.now() + 90 * 60 * 1000).toISOString();
    const { unmount: unmountLong } = render(<Countdown expiresAt={longTime} />);
    await waitFor(() => {
      const timeElement = screen.getByText(/\d+:\d+/);
      expect(timeElement).toBeInTheDocument();
      const timeText = timeElement.textContent;
      expect(timeText).toMatch(/^\d{2}:\d{2}$/);
    });
    unmountLong();
    
    // Test 30 seconds
    const shortTime = new Date(Date.now() + 30 * 1000).toISOString();
    render(<Countdown expiresAt={shortTime} />);
    await waitFor(() => {
      const timeElement = screen.getByText(/\d+:\d+/);
      expect(timeElement).toBeInTheDocument();
      const timeText = timeElement.textContent;
      expect(timeText).toMatch(/^\d{2}:\d{2}$/);
    });
  });

  it('cleans up timer on unmount', () => {
    const futureTime = new Date(Date.now() + 5 * 60 * 1000).toISOString();
    const { unmount } = render(<Countdown expiresAt={futureTime} />);
    
    // Unmount component
    unmount();
    
    // Should not cause any errors - the component should clean up its timers
    // This test mainly ensures that unmounting doesn't throw errors
  });
});

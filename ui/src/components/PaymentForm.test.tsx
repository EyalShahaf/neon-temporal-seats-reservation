import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import PaymentForm from './PaymentForm';

describe('PaymentForm', () => {
  it('disables the submit button when locked', () => {
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={true}
        onSubmit={() => {}}
      />
    );

    const button = screen.getByRole('button', { name: /payment locked/i });
    expect(button).toBeDisabled();
  });

  it('calls onSubmit with the payment code when submitted', () => {
    const handleSubmit = vi.fn();
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={false}
        onSubmit={handleSubmit}
      />
    );

    const input = screen.getByLabelText(/payment code/i);
    const button = screen.getByRole('button', { name: /submit payment/i });

    fireEvent.change(input, { target: { value: '12345' } });
    fireEvent.click(button);

    expect(handleSubmit).toHaveBeenCalledWith('12345');
  });

  it('shows attempts left in the label', () => {
    render(
      <PaymentForm
        attemptsLeft={2}
        lastError=""
        isLocked={false}
        onSubmit={() => {}}
      />
    );

    expect(screen.getByText(/2 attempts left/)).toBeInTheDocument();
  });

  it('shows last error when provided', () => {
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError="Invalid payment code"
        isLocked={false}
        onSubmit={() => {}}
      />
    );

    expect(screen.getByText(/Error: Invalid payment code/)).toBeInTheDocument();
  });

  it('only accepts numeric input', () => {
    const handleSubmit = vi.fn();
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={false}
        onSubmit={handleSubmit}
      />
    );

    const input = screen.getByLabelText(/payment code/i);
    
    // Try to enter non-numeric characters
    fireEvent.change(input, { target: { value: 'abc123def' } });
    
    // Should only contain numbers
    expect(input).toHaveValue('123');
  });

  it('limits input to 5 digits', () => {
    const handleSubmit = vi.fn();
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={false}
        onSubmit={handleSubmit}
      />
    );

    const input = screen.getByLabelText(/payment code/i);
    
    // Try to enter more than 5 digits
    fireEvent.change(input, { target: { value: '123456789' } });
    
    // Should be limited to 5 digits
    expect(input).toHaveValue('12345');
  });

  it('disables submit button when code is not 5 digits', () => {
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={false}
        onSubmit={() => {}}
      />
    );

    const input = screen.getByLabelText(/payment code/i);
    const button = screen.getByRole('button', { name: /submit payment/i });

    // Initially disabled
    expect(button).toBeDisabled();

    // Enter 4 digits - still disabled
    fireEvent.change(input, { target: { value: '1234' } });
    expect(button).toBeDisabled();

    // Enter 5 digits - enabled
    fireEvent.change(input, { target: { value: '12345' } });
    expect(button).not.toBeDisabled();
  });

  it('shows correct button text when locked', () => {
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={true}
        onSubmit={() => {}}
      />
    );

    expect(screen.getByText('Payment Locked')).toBeInTheDocument();
  });

  it('shows correct button text when not locked', () => {
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={false}
        onSubmit={() => {}}
      />
    );

    expect(screen.getByText('Submit Payment')).toBeInTheDocument();
  });

  it('handles form submission via Enter key', () => {
    const handleSubmit = vi.fn();
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={false}
        onSubmit={handleSubmit}
      />
    );

    const input = screen.getByLabelText(/payment code/i);
    const form = input.closest('form');
    
    fireEvent.change(input, { target: { value: '12345' } });
    fireEvent.submit(form!);

    expect(handleSubmit).toHaveBeenCalledWith('12345');
  });

  it('does not submit when locked even with valid code', () => {
    const handleSubmit = vi.fn();
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={true}
        onSubmit={handleSubmit}
      />
    );

    const input = screen.getByLabelText(/payment code/i);
    const form = input.closest('form');
    
    fireEvent.change(input, { target: { value: '12345' } });
    fireEvent.submit(form!);

    expect(handleSubmit).not.toHaveBeenCalled();
  });

  it('clears input when form is submitted', () => {
    const handleSubmit = vi.fn();
    render(
      <PaymentForm
        attemptsLeft={3}
        lastError=""
        isLocked={false}
        onSubmit={handleSubmit}
      />
    );

    const input = screen.getByLabelText(/payment code/i);
    const button = screen.getByRole('button', { name: /submit payment/i });

    fireEvent.change(input, { target: { value: '12345' } });
    fireEvent.click(button);

    expect(handleSubmit).toHaveBeenCalledWith('12345');
    // Input should be cleared after submission
    expect(input).toHaveValue('');
  });
});

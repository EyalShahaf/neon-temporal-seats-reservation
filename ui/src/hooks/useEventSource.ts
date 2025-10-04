import { useState, useEffect, useRef } from 'react';

interface UseEventSourceOptions {
  onMessage?: (event: MessageEvent) => void;
  onError?: (event: Event) => void;
  onOpen?: (event: Event) => void;
}

export function useEventSource<T = any>(url: string, options: UseEventSourceOptions = {}) {
  const [data, setData] = useState<T | null>(null);
  const [readyState, setReadyState] = useState<number>(EventSource.CONNECTING);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!url) return;

    const eventSource = new EventSource(url);
    eventSourceRef.current = eventSource;

    eventSource.onopen = (event) => {
      setReadyState(EventSource.OPEN);
      options.onOpen?.(event);
    };

    eventSource.onmessage = (event) => {
      try {
        const parsedData = JSON.parse(event.data) as T;
        setData(parsedData);
        options.onMessage?.(event);
      } catch (error) {
        console.error('Failed to parse SSE data:', error);
        options.onError?.(event);
      }
    };

    eventSource.onerror = (event) => {
      setReadyState(EventSource.CLOSED);
      options.onError?.(event);
    };

    return () => {
      eventSource.close();
      eventSourceRef.current = null;
    };
  }, [url, options.onMessage, options.onError, options.onOpen]);

  const close = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  };

  return { data, readyState, close };
}
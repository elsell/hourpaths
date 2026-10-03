import { useEffect, useState, type ReactNode } from 'react';

// Mount only while the operation is pending; completion cancels its reveal.
export function DelayedStatus({ children }: { children: ReactNode }) {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    const timer = setTimeout(() => setVisible(true), 1000);
    return () => clearTimeout(timer);
  }, []);
  return visible ? children : null;
}

import type { ReactNode } from "react";

export default function AdminPageTitle({
  children,
  description,
}: {
  children: ReactNode;
  description?: ReactNode;
}) {
  return (
    <div className="min-w-0">
      <h1 className="km-page-title text-xl font-semibold leading-7 text-foreground">
        {children}
      </h1>
      {description ? (
        <p className="mt-1 max-w-3xl text-xs leading-5 text-muted-foreground">
          {description}
        </p>
      ) : null}
    </div>
  );
}

export function AdminSectionTitle({ children }: { children: ReactNode }) {
  return (
    <h2 className="km-kicker text-foreground">
      {children}
    </h2>
  );
}

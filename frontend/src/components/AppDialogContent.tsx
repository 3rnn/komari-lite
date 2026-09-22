import React from "react";
import { Dialog } from "@radix-ui/themes";

type DialogTitleProps = React.ComponentProps<typeof Dialog.Title>;
type DialogDescriptionProps = React.ComponentProps<typeof Dialog.Description>;

export type AppDialogContentProps = Omit<
  React.ComponentProps<typeof Dialog.Content>,
  "children" | "title"
> & {
  title?: React.ReactNode;
  description?: React.ReactNode;
  visuallyHiddenDescription?: React.ReactNode;
  disableDescription?: boolean;
  titleProps?: DialogTitleProps;
  descriptionProps?: DialogDescriptionProps;
  children: React.ReactNode;
};

const joinClassName = (...values: Array<string | undefined>) =>
  values.filter(Boolean).join(" ");

const disabledDescriptionProps = {
  "aria-describedby": undefined,
} as const;

function containsDialogDescription(node: React.ReactNode): boolean {
  let found = false;
  React.Children.forEach(node, (child) => {
    if (found || !React.isValidElement(child)) return;
    if (child.type === Dialog.Description) {
      found = true;
      return;
    }
    const childProps = child.props as { children?: React.ReactNode };
    if (containsDialogDescription(childProps.children)) found = true;
  });
  return found;
}

export default function AppDialogContent({
  title,
  description,
  visuallyHiddenDescription,
  disableDescription = false,
  titleProps,
  descriptionProps,
  children,
  ...contentProps
}: AppDialogContentProps) {
  const hasExplicitAriaDescribedBy = Object.prototype.hasOwnProperty.call(
    contentProps,
    "aria-describedby",
  );
  const {
    "aria-describedby": ariaDescribedBy,
    ...dialogContentProps
  } = contentProps;
  const descriptionContent = disableDescription
    ? null
    : visuallyHiddenDescription ?? description;
  const descriptionClassName = visuallyHiddenDescription
    ? joinClassName("sr-only", descriptionProps?.className)
    : descriptionProps?.className;
  const hasDescriptionContent =
    descriptionContent !== null &&
    descriptionContent !== undefined &&
    descriptionContent !== false &&
    descriptionContent !== "";
  const hasNestedDescription =
    !hasDescriptionContent && containsDialogDescription(children);
  const dialogAccessibilityProps =
    disableDescription ||
    (!hasDescriptionContent && !hasNestedDescription) ||
    (hasExplicitAriaDescribedBy && ariaDescribedBy === undefined)
      ? disabledDescriptionProps
      : hasExplicitAriaDescribedBy
        ? { "aria-describedby": ariaDescribedBy }
        : {};

  return (
    <Dialog.Content
      {...dialogContentProps}
      {...dialogAccessibilityProps}
    >
      {title !== undefined ? (
        <Dialog.Title {...titleProps}>{title}</Dialog.Title>
      ) : null}
      {hasDescriptionContent ? (
        <Dialog.Description
          {...descriptionProps}
          className={descriptionClassName}
        >
          {descriptionContent}
        </Dialog.Description>
      ) : null}
      {children}
    </Dialog.Content>
  );
}

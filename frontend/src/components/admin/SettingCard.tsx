import {
  Button,
  DropdownMenu,
  Flex,
  IconButton,
  Switch,
  TextArea,
  TextField,
} from "@radix-ui/themes";
import React from "react";
import { useTranslation } from "react-i18next";
import { ChevronDownIcon } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useReduceMotionPreference } from "@/lib/api";

interface SettingCardProps {
  title?: string | React.ReactNode;
  description?: string | React.ReactNode;
  children?: React.ReactNode;
  className?: string;
  bordless?: boolean;
  direction?: "row" | "column" | "row-reverse" | "column-reverse";
  onHeaderClick?: () => void;
}

export function SettingCard({
  title = "",
  description = "",
  children,
  className = "",
  direction = "column",
  bordless = false,
  onHeaderClick = () => { },
}: SettingCardProps) {
  const actionChild = React.Children.toArray(children).find(
    (child) => React.isValidElement(child) && child.type === Action
  );

  const otherChildren = React.Children.toArray(children).filter(
    (child) => !(React.isValidElement(child) && child.type === Action)
  );

  return (
    <Flex
      direction={direction}
      justify="between"
      align="center"
      wrap="wrap"
      style={{ borderColor: "var(--gray-a5)" }}
      className={
        bordless
          ? `min-w-0 max-w-full border-0 ${className}`
          : `km-panel min-h-8 min-w-0 max-w-full px-3 py-2 ${className}`
      }
    >
      <Flex
        className="setting-card-header w-full min-w-0 max-w-full gap-3"
        direction="row"
        justify="between"
        align="center"
        wrap="nowrap"
        onClick={onHeaderClick}
      >
        <Flex
          direction="column"
          gap="1"
          className="min-h-10 min-w-0 flex-1"
          justify={"center"}
        >
          <label
            className="min-w-0 break-words text-sm font-medium [overflow-wrap:anywhere]"
            style={{ fontWeight: 600 }}
          >
            {title}
          </label>
          {description && (
            <label className="min-w-0 break-words text-xs text-muted-foreground [overflow-wrap:anywhere]">
              {description}
            </label>
          )}
        </Flex>
        {actionChild ? (
          <div className="setting-card-action min-w-0 shrink-0">
            {actionChild}
          </div>
        ) : null}
      </Flex>
      {otherChildren}
    </Flex>
  );
}

function Action({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}

SettingCard.Action = Action;

export function SettingCardSwitch({
  label = "",
  autoDisabled = true,
  defaultChecked,
  onChange,
  ...props
}: SettingCardProps & {
  label?: string;
  autoDisabled?: boolean;
  defaultChecked?: boolean;
  onChange?: (checked: boolean, switchElement: HTMLButtonElement) => void;
}) {
  const switchRef = React.useRef<HTMLButtonElement>(null);
  const [disabled, setDisabled] = React.useState(false);
  const [checked, setChecked] = React.useState(defaultChecked || false);

  React.useEffect(() => {
    setChecked(Boolean(defaultChecked));
  }, [defaultChecked]);

  const handleChange = (c: boolean) => {
    if (autoDisabled) setDisabled(true);
    const previousValue = checked;
    setChecked(c);
    const result: any =
      onChange && switchRef.current
        ? onChange(c, switchRef.current)
        : undefined;
    if (autoDisabled) {
      const promise: Promise<any> = result;
      if (promise && typeof promise.then === "function") {
        promise
          .then(() => { })
          .catch(() => {
            setChecked(previousValue);
          })
          .finally(() => {
            setDisabled(false);
          });
      } else {
        setDisabled(false);
      }
    }
  };
  return (
    <SettingCard {...props} direction="column">
      <SettingCard.Action>
        <Flex
          direction="row"
          gap="2"
          align="center"
          className="shrink-0 whitespace-nowrap"
        >
          <label className="whitespace-nowrap">{label}</label>
          <Switch
            ref={switchRef}
            checked={checked}
            onCheckedChange={handleChange}
            disabled={disabled}
          />
        </Flex>
      </SettingCard.Action>
    </SettingCard>
  );
}

export function SettingCardButton({
  label = "",
  variant = "solid",
  children,
  onClick,
  autoDisabled = true,
  ...props
}: SettingCardProps & {
  label?: string;
  variant?: "solid" | "soft" | "outline" | "ghost";
  children?: React.ReactNode;
  onClick?: (buttonElement: HTMLButtonElement) => void;
  autoDisabled?: boolean;
}) {
  const [disabled, setDisabled] = React.useState(false);
  const resolvedLabel = label;
  const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (autoDisabled) setDisabled(true);
    const result: any = onClick ? onClick(event.currentTarget) : undefined;
    if (autoDisabled) {
      const promise: Promise<any> = result;
      if (promise && typeof promise.then === "function") {
        promise.finally(() => setDisabled(false)).catch(() => {});
      } else {
        setDisabled(false);
      }
    }
  };
  return (
    <SettingCard {...props} direction="column">
      <SettingCard.Action>
        <Flex>
          <Flex direction="row" gap="2" align="center">
            <label>{resolvedLabel}</label>
            <Button onClick={handleClick} variant={variant} disabled={disabled}>
              {children}
            </Button>
          </Flex>
        </Flex>
      </SettingCard.Action>
    </SettingCard>
  );
}

export function SettingCardIconButton({
  label = "",
  variant = "solid",
  children,
  onClick,
  autoDisabled = true,
  ...props
}: SettingCardProps & {
  label?: string;
  variant?: "solid" | "soft" | "outline" | "ghost";
  children?: React.ReactNode;
  onClick?: (buttonElement: HTMLButtonElement) => void;
  autoDisabled?: boolean;
}) {
  const [disabled, setDisabled] = React.useState(false);
  const resolvedLabel = label;
  const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (autoDisabled) setDisabled(true);
    const result: any = onClick ? onClick(event.currentTarget) : undefined;
    if (autoDisabled) {
      const promise: Promise<any> = result;
      if (promise && typeof promise.then === "function") {
        promise.finally(() => setDisabled(false)).catch(() => {});
      } else {
        setDisabled(false);
      }
    }
  };
  return (
    <SettingCard {...props} direction="column">
      <SettingCard.Action>
        <Flex>
          <Flex direction="row" gap="2" align="center">
            <label>{resolvedLabel}</label>
            <IconButton
              onClick={handleClick}
              variant={variant}
              disabled={disabled}
            >
              {children}
            </IconButton>
          </Flex>
        </Flex>
      </SettingCard.Action>
    </SettingCard>
  );
}

interface SettingCardShortTextInputProps
  extends Omit<React.ComponentProps<typeof TextField.Root>, 'onChange' | 'onKeyDown'> {
  // SettingCard props.
  title?: string;
  description?: string;
  descriptionPlacement?: "header" | "footer";
  bordless?: boolean;

  // Button props.
  showSaveButton?: boolean;
  label?: string;
  autoDisabled?: boolean;
  isSaving?: boolean;

  // Save callback.
  OnSave?: (
    value: string,
    inputElement: HTMLInputElement,
    buttonElement: HTMLButtonElement
  ) => void | Promise<unknown>;

  // Additional content.
  children?: React.ReactNode | null;

  // Optional input event callback.
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
}

export function SettingCardShortTextInput({
  // SettingCard props.
  title = "",
  description = "",
  descriptionPlacement = "header",
  bordless = false,

  // Button props.
  showSaveButton = true,
  label = "",
  autoDisabled = true,
  isSaving,

  // Save callback.
  OnSave = () => { },

  // Additional content.
  children = null,

  // Event callback.
  onChange,
  onKeyDown,

  // All other TextField.Root props.
  value,
  defaultValue,
  placeholder,
  disabled,
  type = "text",
  required,
  readOnly,
  maxLength,
  minLength,
  pattern,
  autoComplete,
  autoFocus,
  name,
  id,
  className = "w-full",
  ...restProps
}: SettingCardShortTextInputProps) {
  const { t } = useTranslation();
  const [internalDisabled, setInternalDisabled] = React.useState(false);
  const savingState = Boolean(isSaving) || internalDisabled;
  const normalizedValue =
    value !== undefined && value !== null ? String(value) : "";
  const normalizedDefaultValue =
    defaultValue !== undefined && defaultValue !== null
      ? String(defaultValue)
      : "";
  const [internalValue, setInternalValue] = React.useState(
    value !== undefined ? normalizedValue : normalizedDefaultValue
  );
  const previousDefaultValueRef = React.useRef(normalizedDefaultValue);
  const currentValue = value !== undefined ? normalizedValue : internalValue;
  const inputRef = React.useRef<HTMLInputElement>(null);
  const buttonRef = React.useRef<HTMLButtonElement>(null);
  const resolvedLabel = label || t("save");

  // Sync internal state when the external value changes.
  React.useEffect(() => {
    if (value !== undefined) {
      setInternalValue(normalizedValue);
      previousDefaultValueRef.current = normalizedDefaultValue;
      return;
    }

    if (normalizedDefaultValue !== previousDefaultValueRef.current) {
      const previousDefaultValue = previousDefaultValueRef.current;
      setInternalValue((currentInternalValue) =>
        currentInternalValue === previousDefaultValue
          ? normalizedDefaultValue
          : currentInternalValue
      );
      previousDefaultValueRef.current = normalizedDefaultValue;
    }
  }, [normalizedDefaultValue, normalizedValue, value]);

  const handleSave = () => {
    if (autoDisabled) setInternalDisabled(true);
    const valueToSave = currentValue?.toString() || "";
    const result: any =
      inputRef.current && buttonRef.current
        ? OnSave(valueToSave, inputRef.current, buttonRef.current)
        : undefined;
    if (autoDisabled) {
      const promise: Promise<any> = result;
      if (promise && typeof promise.then === "function") {
        promise
          .finally(() => setInternalDisabled(false))
          .catch(() => {});
      } else {
        setInternalDisabled(false);
      }
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;

    // Update internal state only in uncontrolled mode.
    if (value === undefined) {
      setInternalValue(newValue);
    }

    // Invoke the external onChange callback.
    onChange?.(e);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    // Save on Enter.
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSave();
    }

    // Invoke the external onKeyDown callback.
    onKeyDown?.(e);
  };

  const showFooterDescription =
    descriptionPlacement === "footer" &&
    description !== undefined &&
    description !== null &&
    description !== "";
  const showFooterRow =
    descriptionPlacement === "footer" &&
    (showFooterDescription || showSaveButton);
  const showHiddenFooterSaveButton =
    descriptionPlacement === "footer" &&
    !showFooterDescription &&
    !showSaveButton;

  return (
    <SettingCard
      title={title}
      description={descriptionPlacement === "footer" ? undefined : description}
      bordless={bordless}
    >
      <Flex direction="column" className="w-full mt-1" gap="2" align="start">
        <TextField.Root
          {...restProps}
          className={className}
          value={currentValue}
          defaultValue={value === undefined ? normalizedDefaultValue : undefined}
          placeholder={placeholder}
          disabled={disabled || savingState}
          type={type}
          required={required}
          readOnly={readOnly}
          maxLength={maxLength}
          minLength={minLength}
          pattern={pattern}
          autoComplete={autoComplete}
          autoFocus={autoFocus}
          name={name}
          id={id}
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
          ref={inputRef}
        >
        </TextField.Root>
        {children}
        {descriptionPlacement === "footer" ? (
          showFooterRow ? (
            <Flex
              direction="row"
              className="w-full"
              align="center"
              justify={showFooterDescription ? "between" : "end"}
              gap="3"
            >
              {showFooterDescription ? (
                <label className="min-w-0 flex-1 text-sm text-muted-foreground">
                  {description}
                </label>
              ) : null}
              <Button
                ref={buttonRef}
                onClick={handleSave}
                variant="solid"
                hidden={!showSaveButton}
                disabled={savingState}
              >
                {resolvedLabel}
              </Button>
            </Flex>
          ) : showHiddenFooterSaveButton ? (
            <Button
              ref={buttonRef}
              onClick={handleSave}
              variant="solid"
              hidden
              disabled={savingState}
            >
              {resolvedLabel}
            </Button>
          ) : null
        ) : (
          <Button
            ref={buttonRef}
            onClick={handleSave}
            variant="solid"
            hidden={!showSaveButton}
            disabled={savingState}
          >
            {resolvedLabel}
          </Button>
        )}
      </Flex>
    </SettingCard>
  );
}

export function SettingCardLongTextInput({
  title = "",
  description = "",
  descriptionPlacement = "header",
  label = "",
  defaultValue = "",
  OnSave = () => { },
  onChange,
  autoDisabled = true,
  isSaving,
  bordless = false,
  showSaveButton = true,
}: {
  title?: string;
  description?: string;
  descriptionPlacement?: "header" | "footer";
  label?: string;
  defaultValue?: string;
  OnSave?: (
    value: string,
    textAreaElement: HTMLTextAreaElement,
    buttonElement: HTMLButtonElement
  ) => void | Promise<unknown>;
  onChange?: (e: React.ChangeEvent<HTMLTextAreaElement>) => void;
  autoDisabled?: boolean;
  isSaving?: boolean;
  bordless?: boolean;
  showSaveButton?: boolean;
}) {
  const { t } = useTranslation();
  const [disabled, setDisabled] = React.useState(false);
  const savingState = Boolean(isSaving) || disabled;
  const [value, setValue] = React.useState(defaultValue);
  const textAreaRef = React.useRef<HTMLTextAreaElement>(null);
  const buttonRef = React.useRef<HTMLButtonElement>(null);
  const resolvedLabel = label || t("save");

  React.useEffect(() => {
    setValue(defaultValue);
  }, [defaultValue]);

  const handleSave = () => {
    if (autoDisabled) setDisabled(true);
    const result: any =
      textAreaRef.current && buttonRef.current
        ? OnSave(value, textAreaRef.current, buttonRef.current)
        : undefined;
    if (autoDisabled) {
      const promise: Promise<any> = result;
      if (promise && typeof promise.then === "function") {
        promise.finally(() => setDisabled(false)).catch(() => {});
      } else {
        setDisabled(false);
      }
    }
  };

  const handleTextAreaChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setValue(e.target.value);
    // Invoke the external onChange callback.
    onChange?.(e);
  };

  const showFooterDescription =
    descriptionPlacement === "footer" &&
    description !== undefined &&
    description !== null &&
    description !== "";

  return (
    <SettingCard
      title={title}
      description={descriptionPlacement === "footer" ? undefined : description}
      bordless={bordless}
    >
      <Flex direction="column" className="w-full mt-1" gap="2" align="start">
        <TextArea
          className="w-full"
          defaultValue={defaultValue}
          resize="vertical"
          value={value}
          onChange={handleTextAreaChange}
          ref={textAreaRef}
        />
        {descriptionPlacement === "footer" ? (
          showFooterDescription || showSaveButton ? (
            <Flex
              direction="row"
              className="w-full"
              align="center"
              justify={showFooterDescription ? "between" : "end"}
              gap="3"
            >
              {showFooterDescription ? (
                <label className="min-w-0 flex-1 text-sm text-muted-foreground">
                  {description}
                </label>
              ) : null}
              {showSaveButton ? (
                <Button
                  ref={buttonRef}
                  onClick={handleSave}
                  variant="solid"
                  disabled={savingState}
                >
                  {resolvedLabel}
                </Button>
              ) : null}
            </Flex>
          ) : null
        ) : showSaveButton ? (
          <Button
            ref={buttonRef}
            onClick={handleSave}
            variant="solid"
            disabled={savingState}
          >
            {resolvedLabel}
          </Button>
        ) : null}
      </Flex>
    </SettingCard>
  );
}

export function SettingCardSelect({
  title,
  description,
  defaultValue = "",
  value,
  label = "",
  options = [],
  OnSave = () => { },
  autoDisabled = true,
  isSaving,
  bordless = false,
}: {
  title?: string;
  description?: string;
  defaultValue?: string;
  value?: string;
  label?: string;
  options?: { value: string; label?: string; disabled?: boolean }[];
  OnSave?: (value: string, buttonElement: HTMLButtonElement) => void;
  autoDisabled?: boolean;
  isSaving?: boolean;
  bordless?: boolean;
}) {
  const { t } = useTranslation();
  const [disabled, setDisabled] = React.useState(false);
  const savingState = isSaving !== undefined ? isSaving : disabled;
  const [selectedValue, setSelectedValue] = React.useState(
    value !== undefined ? value : defaultValue
  );
  const buttonRef = React.useRef<HTMLButtonElement>(null);
  const resolvedLabel = label || t("select");

  React.useEffect(() => {
    if (value !== undefined) {
      setSelectedValue(value);
    }
  }, [value]);

  const handleSave = (value: string) => {
    if (isSaving === undefined && autoDisabled) setDisabled(true);
    const previousValue = selectedValue; // Remember the prior value.
    setSelectedValue(value); // Show the newly selected value.

    const result: any = buttonRef.current
      ? OnSave(value, buttonRef.current)
      : undefined;
    if (autoDisabled) {
      const promise: Promise<any> = result;
      if (promise && typeof promise.then === "function") {
        promise
          .then(() => {
            // The selected value is already updated after a successful save.
          })
          .catch(() => {
            // Revert to the previous value on error.
            setSelectedValue(previousValue);
          })
          .finally(() => {
            if (isSaving === undefined) {
              setDisabled(false);
            }
          });
      } else {
        if (isSaving === undefined) {
          setDisabled(false);
        }
      }
    }
  };

  // Prefer the selected option's label for display.
  const getDisplayText = () => {
    if (selectedValue) {
      const selectedOption = options.find(
        (option) => option.value === selectedValue
      );
      return selectedOption?.label || selectedValue;
    }
    return resolvedLabel;
  };

  return (
    <SettingCard title={title} description={description} bordless={bordless}>
      <SettingCard.Action>
        <Flex>
          <Flex direction="row" gap="2" align="center">
            <DropdownMenu.Root>
              <DropdownMenu.Trigger disabled={savingState}>
                <Button variant="soft" ref={buttonRef}>
                  {getDisplayText()}
                  <DropdownMenu.TriggerIcon />
                </Button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Content>
                {options.map((option) => (
                  <DropdownMenu.Item
                    disabled={option.disabled}
                    key={option.value}
                    onSelect={() => {
                      handleSave(option.value);
                    }}
                  >
                    {option.label ? option.label : option.value}
                  </DropdownMenu.Item>
                ))}
              </DropdownMenu.Content>
            </DropdownMenu.Root>
          </Flex>
        </Flex>
      </SettingCard.Action>
    </SettingCard>
  );
}

export function SettingCardLabel({
  children,
}: {
  children: React.ReactNode | null;
}) {
  return (
    <label className="text-base font-semibold leading-6 text-foreground">
      {children}
    </label>
  );
}

export function SettingCardCollapse({
  title,
  description,
  defaultOpen = false,
  children,
  bordless = false,
}: {
  title?: string;
  description?: string;
  children?: React.ReactNode;
  defaultOpen?: boolean;
  bordless?: boolean;
}) {
  const [open, setOpen] = React.useState(defaultOpen);
  const reduceMotion = useReduceMotionPreference();

  return (
    <SettingCard
      title={title}
      description={description}
      onHeaderClick={() => setOpen(!open)}
      bordless={bordless}
    >
      <SettingCard.Action>
        <IconButton
          variant="soft"
          onClick={() => setOpen(!open)}
          aria-expanded={open}
          aria-controls="collapsible-content"
        >
          <motion.div
            initial={reduceMotion ? false : { rotate: 0, scale: 1 }}
            animate={{ rotate: open ? 180 : 0, scale: open ? 1.1 : 1 }}
            transition={
              reduceMotion
                ? { duration: 0 }
                : { duration: 0.25, ease: [0.4, 0, 0.2, 1] }
            }
          >
            <ChevronDownIcon />
          </motion.div>
        </IconButton>
      </SettingCard.Action>
      <AnimatePresence>
        {open && (
          <motion.div
            className="w-full p-0 md:p-1" // Ensures the content takes full width
            layout={!reduceMotion}
            initial={reduceMotion ? false : { height: 0, opacity: 0, y: -10 }}
            animate={{ height: "auto", opacity: 1, y: 0 }}
            exit={{ height: 0, opacity: 0, y: -10 }}
            transition={
              reduceMotion
                ? { duration: 0 }
                : { duration: 0.25, ease: [0.4, 0, 0.2, 1] }
            }
            style={{ overflow: "hidden" }} // Prevents content clipping during animation
            id="collapsible-content"
          >
            <div className="border-t-1 my-2" />
            {children}
          </motion.div>
        )}
      </AnimatePresence>
    </SettingCard>
  );
}

// Header slot for SettingCardCollapse
SettingCardCollapse.Header = function Header({
  children,
}: {
  children: React.ReactNode;
}) {
  return <div>{children}</div>;
};
